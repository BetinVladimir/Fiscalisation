package dossier

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFiscalEvidencePostgres(t *testing.T) {
	adminURL, readerURL := os.Getenv("DOSSIER_EVIDENCE_TEST_ADMIN_URL"), os.Getenv("DOSSIER_EVIDENCE_TEST_READER_URL")
	if adminURL == "" || readerURL == "" {
		t.Skip("actual disposable migrated PostgreSQL required; use dossier-evidence-postgres-integration.sh")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	reader, err := pgx.Connect(ctx, readerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close(ctx)
	resources := []struct{ kind, id, tenant, data string }{
		{"operator", "operator-evidence", "tenant-evidence", `{"code":"A001","first_name":"Иван","last_name":"Иванов","roles":["ADMIN"],"active_from":"2026-10-18T00:00:00Z","password":"OPERATOR_SECRET","token":"BEARER_SECRET"}`},
		{"device", "device-evidence", "tenant-evidence", `{"kind":"EDGE","vendor":"BEELOY","model":"S3","serial":"BOARD001","status":"RETIRED","credential_id":"credential-id-only","private_key":"DEVICE_KEY_SECRET","mqtt_password":"MQTT_SECRET"}`},
		{"execution_authorization", "authorization-evidence", "tenant-evidence", `{"issued_binding_sha256":"binding-hash","envelope":{"algorithm":"ES256_DER","key_id":"issuer","sha256":"authorization-hash","payload":"BASE64_GRANT_SECRET","signature":"GRANT_SIGNATURE_SECRET"},"issuance_sale_id":"sale-evidence","issuance_operation_id":"operation-evidence","issuance_sale_version":9007199254740993,"issuance_operation_version":9223372036854775807,"token":"AUTH_SECRET"}`},
		{"execution_authorization_revocation", "authorization-evidence", "tenant-evidence", `{"effective_at":"2026-10-18T01:00:00Z","actor_id":"admin-1","reason_code":"OPERATOR_REVOKED","token":"REVOCATION_SECRET"}`},
		{"issued_binding", "binding-evidence", "tenant-evidence", `{"payload_sha256":"binding-hash","key_id":"binding-key","issued_at":"2026-10-18T00:00:00Z","canonical_json":"CANONICAL_WITH_KEY_SECRET","signature":"BINDING_SIGNATURE_SECRET","canonical_payload":{"schema_version":3,"generation":9007199254740993,"tenant_id":"tenant-evidence","register_id":"register-1","edge_device_id":"device-evidence","fiscal_endpoint":{"device_id":"fiscal-1","fiscal_device_number":"DT000001","fiscal_memory_number":"12345678","password":"FISCAL_KEY_SECRET"},"operational_authority":{"unp_prefix":"DT000001","unp_range_start":1,"unp_range_end":1000,"hmac_key":"HMAC_SECRET"},"mqtt":{"private_key":"NESTED_KEY_SECRET"}}}`},
		{"payment_reconciliation", "payment-evidence", "tenant-evidence", `{"grant_sha256":"grant-hash","original_operation_id":"operation-evidence","issued_resource_digest":"issued-digest","issuance_pending_digest":"pending-digest","event":{"event_id":"reconciliation-1-reconciled","operation_id":"operation-evidence","device_id":"device-evidence","journal_seq":9007199254740993,"event_type":"PAYMENT_RECONCILED","occurred_at":"2026-10-18T02:00:00Z","event_hash":"event-hash","signature":"EVENT_SIGNATURE_SECRET","payload":{"operation_id":"operation-evidence","payment_id":"payment-a","old_state":"UNKNOWN","new_state":"APPROVED","amount_minor":"9007199254740993","rrn":"123456789012","auth_code":"AUTH-CODE-IS-REFERENCE","terminal_reference":"STAN-123","authorization_sha256":"grant-hash","evidence_reference":"evidence-1","token":"PAYMENT_TOKEN_SECRET"}}}`},
		{"payment_reconciliation_authorization", "payment-evidence", "tenant-evidence", `{"issuance_operation_id":"operation-evidence","issuance_pending_digest":"pending-digest","issuance_binding_id":"binding-evidence","issuance_binding_version":9007199254740993,"envelope":{"sha256":"grant-hash","key_id":"issuer","algorithm":"ES256_DER","payload":"RECONCILIATION_GRANT_SECRET","signature":"RECONCILIATION_SIGNATURE_SECRET"}}`},
		{"future_unknown_kind", "unknown-evidence", "tenant-evidence", `{"private_key":"UNKNOWN_PRIVATE_SECRET","token":"UNKNOWN_TOKEN_SECRET"}`},
		{"provisioning_session", "provisioning-evidence", "tenant-evidence", `{"wifi_password":"WIFI_SECRET","package":"PROVISIONING_SECRET"}`},
		{"operator", "foreign-operator", "foreign-evidence", `{"code":"B001","first_name":"FOREIGN_SECRET","roles":["CASHIER"]}`},
	}
	for _, r := range resources {
		_, err = admin.Exec(ctx, `insert into fiscal.fiscal_runtime_resources(kind,id,tenant_id,version,data,created_at,updated_at,payload) values($1,$2,$3,1,$4::jsonb,now(),now(),'{"duplicate":"RAW_RESOURCE_PAYLOAD_SECRET"}') on conflict(kind,id) do update set data=excluded.data,version=1`, r.kind, r.id, r.tenant, r.data)
		if err != nil {
			t.Fatal(err)
		}
	}
	audits := []struct{ id, kind, action, tenant, before, after string }{
		{"audit-operation", "operation", "FISCAL_OPERATION_RESULT", "tenant-evidence", `{"operation_id":"operation-evidence","state":"UNKNOWN","version":9007199254740993,"token":"BEFORE_SECRET"}`, `{"operation_id":"operation-evidence","client_operation_id":"client-op-1","receipt_session_id":"receipt-session-1","state":"FISCALIZED","version":9007199254740994,"simulated":false,"fiscal_reference":"receipt-ref-1","token":"AFTER_SECRET"}`},
		{"audit-operator", "operator", "OPERATOR_ROLE_CHANGED", "tenant-evidence", `{"code":"A001","roles":["CASHIER"],"token":"OLD_OPERATOR_SECRET"}`, `{"code":"A001","roles":["ADMIN"],"password":"NEW_OPERATOR_SECRET"}`},
		{"audit-revocation", "execution_authorization_revocation", "CONFIGURATION_CHANGED", "tenant-evidence", `null`, `{"effective_at":"2026-10-18T01:00:00Z","actor_id":"admin-1","token":"AUDIT_REVOCATION_SECRET"}`},
		{"audit-device", "device", "SMART_DEVICE_REVOKED", "tenant-evidence", `null`, `{"kind":"device","tenant_id":"tenant-evidence","id":"device-evidence","version":2,"data":{"status":"RETIRED","serial":"BOARD001","credential_id":"credential-id-only","private_key":"AUDIT_DEVICE_KEY_SECRET"}}`},
		{"audit-payment", "payment_reconciliation", "PAYMENT_RECONCILED", "tenant-evidence", `null`, resources[5].data},
		{"audit-unknown", "future_unknown_kind", "NEW_FUTURE_ACTION", "tenant-evidence", `{"token":"UNKNOWN_BEFORE_SECRET"}`, `{"private_key":"UNKNOWN_AFTER_SECRET"}`},
		{"audit-provisioning", "provisioning_session", "CONFIGURATION_CHANGED", "tenant-evidence", `null`, `{"private_key":"PROVISIONING_AUDIT_SECRET"}`},
		{"audit-foreign", "operator", "OPERATOR_ROLE_CHANGED", "foreign-evidence", `null`, `{"first_name":"FOREIGN_AUDIT_SECRET"}`},
	}
	for _, a := range audits {
		payload := `{"event_id":"` + a.id + `","tenant_id":"` + a.tenant + `","actor_id":"admin-1","action":"` + a.action + `","object_type":"` + a.kind + `","object_id":"object-evidence","unp":"DT000001-A001-0000001","previous_hash":"` + strings.Repeat("a", 64) + `","before":` + a.before + `,"after":` + a.after + `,"private_key":"ROOT_AUDIT_SECRET"}`
		_, err = admin.Exec(ctx, `insert into fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload) values($1,$2,'admin-1',$3,$4,'object-evidence',now(),$5,$6::jsonb) on conflict(event_id) do nothing`, a.id, a.tenant, a.action, a.kind, strings.Repeat("b", 64), payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	tx, err := reader.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	contract, err := InspectProfile(ctx, tx, "fiscal", "fiscal-evidence")
	tx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reviewedRaw, err := os.ReadFile("../../../docs/remediation/2026-10-17/28_FISCAL_EVIDENCE_PG_DICTIONARY.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	var reviewed Contract
	if err = json.Unmarshal(reviewedRaw, &reviewed); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(contract)
	want, _ := json.Marshal(reviewed)
	if string(got) != string(want) {
		t.Fatal("evidence dictionary drift")
	}
	options := func(out string) Options {
		return Options{Profile: "fiscal-evidence", Schema: "fiscal", Tenant: "tenant-evidence", Output: out, SourceCommit: "synthetic-fixture", Contract: contract}
	}
	t.Run("secrets_pruned_before_transport_and_immutable_evidence_retained", func(t *testing.T) {
		// Direct SQL expression test checks the DB-returned bytes before any Go redaction.
		for _, table := range []string{"fiscal_runtime_resources", "fiscal_runtime_audit"} {
			column := "data"
			if table == "fiscal_runtime_audit" {
				column = "payload"
			}
			tx, e := reader.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
			if e != nil {
				t.Fatal(e)
			}
			tx.Exec(ctx, `select set_config('app.tenant_id','tenant-evidence',true)`)
			rows, e := tx.Query(ctx, "select "+evidenceExpression(table, column)+" from fiscal."+table)
			if e != nil {
				tx.Rollback(ctx)
				t.Fatal(e)
			}
			for rows.Next() {
				var raw string
				if e = rows.Scan(&raw); e != nil {
					t.Fatal(e)
				}
				if strings.Contains(raw, "_SECRET") {
					t.Fatal("SQL returned secret before Go projection")
				}
			}
			e = rows.Err()
			rows.Close()
			tx.Rollback(ctx)
			if e != nil {
				t.Fatal(e)
			}
		}
		out := filepath.Join(t.TempDir(), "evidence")
		if err := Export(ctx, reader, options(out)); err != nil {
			t.Fatal(err)
		}
		if err := Verify(out); err != nil {
			t.Fatal(err)
		}
		var m Manifest
		raw, _ := os.ReadFile(filepath.Join(out, "manifest.json"))
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if m.Coverage["fiscal_runtime_resources:future_unknown_kind:EXCLUDED_UNREVIEWED_KIND"] != 1 || m.Coverage["fiscal_runtime_resources:provisioning_session:EXCLUDED_CREDENTIAL_KIND"] != 1 {
			t.Fatal("unknown/excluded inventory missing", m.Coverage)
		}
		if m.Coverage["fiscal_runtime_audit:operation:PROJECTED_ALLOWLIST_V1"] != 1 || m.Coverage["fiscal_runtime_audit:operation:PROJECTED_HASH_POINTER_MISMATCH"] != 0 {
			t.Fatal("canonical previous_hash source/typed values disagree", m.Coverage)
		}
		all := ""
		entries, _ := os.ReadDir(out)
		for _, entry := range entries {
			body, _ := os.ReadFile(filepath.Join(out, entry.Name()))
			all += string(body)
		}
		if strings.Contains(all, "_SECRET") || strings.Contains(all, "foreign-evidence") {
			t.Fatal("secret/tenant leak")
		}
		for _, proof := range []string{"9007199254740993", "9223372036854775807", "payment-a", "APPROVED", "123456789012", "AUTH-CODE-IS-REFERENCE", "OPERATOR_ROLE_CHANGED", "SMART_DEVICE_REVOKED", "effective_at", "receipt-session-1", "DT000001", "12345678"} {
			if !strings.Contains(all, proof) {
				t.Fatal("required evidence lost", proof)
			}
		}
		hashes := m.Files
		delete(m.Coverage, "fiscal_runtime_resources:future_unknown_kind:EXCLUDED_UNREVIEWED_KIND")
		modified, _ := json.Marshal(m)
		os.WriteFile(filepath.Join(out, "manifest.json"), modified, 0600)
		if err := Verify(out); err == nil {
			t.Fatal("coverage inventory tamper accepted")
		}
		if len(hashes) != 3 {
			t.Fatal("unexpected file inventory")
		}
	})
	t.Run("snapshot_concurrent_resource_and_audit_insert", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "snapshot")
		concurrentID := fmt.Sprintf("audit-concurrent-%d", time.Now().UnixNano())
		err := export(ctx, reader, options(out), func() error {
			_, e := admin.Exec(ctx, `update fiscal.fiscal_runtime_resources set version=2 where kind='operator' and id='operator-evidence';insert into fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload) values('`+concurrentID+`','tenant-evidence','admin-1','OPERATOR_CHANGED','operator','operator-evidence',now(),repeat('c',64),'{"after":{"code":"A001"}}') on conflict do nothing`)
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := os.ReadFile(filepath.Join(out, "fiscal_runtime_audit.jsonl"))
		if strings.Contains(string(body), concurrentID) {
			t.Fatal("later audit leaked into earlier snapshot")
		}
		body, _ = os.ReadFile(filepath.Join(out, "fiscal_runtime_resources.jsonl"))
		if !strings.Contains(string(body), `"operator-evidence","tenant-evidence","1"`) {
			t.Fatal("mixed resource version")
		}
	})
	t.Run("unknown_kinds_never_expand_raw_data", func(t *testing.T) {
		_, err := admin.Exec(ctx, `insert into fiscal.fiscal_runtime_resources(kind,id,tenant_id,version,data,created_at,updated_at) values('newer_unreviewed','brand-new','tenant-evidence',1,'{"access_token":"BRAND_NEW_SECRET"}',now(),now()) on conflict do nothing`)
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "unknown")
		if err = Export(ctx, reader, options(out)); err != nil {
			t.Fatal(err)
		}
		body, _ := os.ReadFile(filepath.Join(out, "fiscal_runtime_resources.jsonl"))
		if strings.Contains(string(body), "BRAND_NEW_SECRET") || !strings.Contains(string(body), "EXCLUDED_UNREVIEWED_KIND") {
			t.Fatal("unknown kind expanded")
		}
	})
	t.Run("malformed_allowlisted_value_fails_without_contents", func(t *testing.T) {
		_, err := admin.Exec(ctx, `update fiscal.fiscal_runtime_resources set data=jsonb_set(data,'{code}','{"private_key":"MALFORMED_KEY_SECRET"}') where kind='operator' and id='operator-evidence'`)
		if err != nil {
			t.Fatal(err)
		}
		defer admin.Exec(ctx, `update fiscal.fiscal_runtime_resources set data=jsonb_set(data,'{code}','"A001"') where kind='operator' and id='operator-evidence'`)
		out := filepath.Join(t.TempDir(), "bad-type")
		err = Export(ctx, reader, options(out))
		if err == nil || strings.Contains(err.Error(), "MALFORMED_KEY_SECRET") {
			t.Fatal("malformed value accepted/leaked", err)
		}
		if _, e := os.Stat(out); !os.IsNotExist(e) {
			t.Fatal("partial output")
		}
	})
	t.Run("schema_drift_and_readonly", func(t *testing.T) {
		_, err := admin.Exec(ctx, `alter table fiscal.fiscal_runtime_resources add column dossier_drift text`)
		if err != nil {
			t.Fatal(err)
		}
		if err = Export(ctx, reader, options(filepath.Join(t.TempDir(), "drift"))); err == nil {
			t.Fatal("drift accepted")
		}
		admin.Exec(ctx, `alter table fiscal.fiscal_runtime_resources drop column dossier_drift`)
		err = export(ctx, reader, options(filepath.Join(t.TempDir(), "readonly")), func() error {
			_, e := reader.Exec(ctx, `update fiscal.fiscal_runtime_resources set version=3 where id='operator-evidence'`)
			if e == nil {
				t.Fatal("write allowed")
			}
			return e
		})
		if err == nil {
			t.Fatal("readonly failure swallowed")
		}
	})
	t.Run("audit_is_append_only_actual_db_and_export_no_changes", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `update fiscal.fiscal_runtime_audit set actor_id='other' where event_id='audit-operator'`); err == nil {
			t.Fatal("actual audit mutable")
		}
		query := `select md5(string_agg(row_to_json(t)::text,'' order by kind,id)) from fiscal.fiscal_runtime_resources t`
		var before, after string
		admin.QueryRow(ctx, query).Scan(&before)
		if err := Export(ctx, reader, options(filepath.Join(t.TempDir(), "unchanged"))); err != nil {
			t.Fatal(err)
		}
		admin.QueryRow(ctx, query).Scan(&after)
		if before != after {
			t.Fatal("source mutated")
		}
	})
}
