package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMiniPOSStateRowsRoundTrip(t *testing.T) {
	raw := []byte(`{"products":{"p":{"id":"p"}},"employees":{},"shifts":{},"orders":{"o":{"id":"o"}},"checkouts":{},"checkout_hashes":{"k":"h"},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":42}`)
	rows, e := flattenSnapshot(raw)
	if e != nil {
		t.Fatal(e)
	}
	rebuilt, e := rebuildSnapshot(rows)
	if e != nil {
		t.Fatal(e)
	}
	var got, want map[string]any
	_ = json.Unmarshal(rebuilt, &got)
	_ = json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip changed state: %#v %#v", want, got)
	}
}

func TestMiniPOSIdentityBindingTypedPersistenceAndRLS(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from minipos_runtime_operator_sessions;delete from minipos_runtime_identity_bindings;delete from minipos_runtime_employees`); err != nil {
		t.Fatal(err)
	}
	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	bindingKey := "org-auth\n" + hash
	sessionKey := "org-auth\nfingerprint-1"
	employee := map[string]any{"id": "employee-auth", "tenant_id": "org-auth", "first_name": "Ada", "last_name": "Lovelace", "operator_code": "A001", "roles": []string{"CASHIER"}, "active": true, "status": "ACTIVE", "version": 1, "created_at": "2026-08-09T10:00:00Z", "updated_at": "2026-08-09T10:00:00Z"}
	binding := map[string]any{"tenant_id": "org-auth", "employee_id": "employee-auth", "subject_hash": hash, "identity_issuer": "https://identity.example.test", "bound_at": "2026-08-09T10:00:00Z"}
	session := map[string]any{"tenant_id": "org-auth", "employee_id": "employee-auth", "app_instance_id": "00000000-0000-4000-8000-000000000001", "token_hash": "fingerprint-1", "state": "REVOKED", "first_seen": "2026-08-09T10:00:00Z", "expires_at": "2026-08-09T11:00:00Z", "revoked_at": "2026-08-09T10:05:00Z"}
	state := map[string]any{"products": map[string]any{}, "employees": map[string]any{"employee-auth": employee}, "identity_bindings": map[string]any{bindingKey: binding}, "operator_sessions": map[string]any{sessionKey: session}, "shifts": map[string]any{}, "orders": map[string]any{}, "checkouts": map[string]any{}, "checkout_hashes": map[string]any{}, "api_replays": map[string]any{}, "webhook_inbox": map[string]any{}, "configurations": map[string]any{}, "sequence": 1}
	raw, _ := json.Marshal(state)
	if err = p.Save(raw); err != nil {
		t.Fatal(err)
	}
	var tenant, employeeID, issuer, persistedHash string
	if err = p.db.QueryRow(`select organization_id,employee_id,identity_issuer,subject_hash from minipos_runtime_identity_bindings where binding_key=$1`, bindingKey).Scan(&tenant, &employeeID, &issuer, &persistedHash); err != nil || tenant != "org-auth" || employeeID != "employee-auth" || issuer != "https://identity.example.test" || persistedHash != hash {
		t.Fatalf("typed binding mismatch: %q %q %q %q %v", tenant, employeeID, issuer, persistedHash, err)
	}
	var sessionState, fingerprint, appInstance string
	if err = p.db.QueryRow(`select state,credential_fingerprint,app_instance_id from minipos_runtime_operator_sessions where session_key=$1`, sessionKey).Scan(&sessionState, &fingerprint, &appInstance); err != nil || sessionState != "REVOKED" || fingerprint != "fingerprint-1" || appInstance != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("typed session mismatch: %q %q %q %v", sessionState, fingerprint, appInstance, err)
	}
	restarted, _, err := p.LoadVersioned()
	if err != nil || !bytes.Contains(restarted, []byte(`"identity_bindings"`)) || !bytes.Contains(restarted, []byte(`"operator_sessions"`)) || !bytes.Contains(restarted, []byte(hash)) {
		t.Fatalf("binding did not survive typed restart: %s %v", restarted, err)
	}
	tx, err := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`set local role beeminipos_tenant`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`select set_config('app.organization_id','org-other',true)`); err != nil {
		t.Fatal(err)
	}
	var visible int
	if err = tx.QueryRow(`select count(*) from minipos_runtime_identity_bindings`).Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("cross-tenant binding visible: %d %v", visible, err)
	}
	if err = tx.QueryRow(`select count(*) from minipos_runtime_operator_sessions`).Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("cross-tenant operator session visible: %d %v", visible, err)
	}
}

func TestMiniPOSDifferentialSavePreservesUntouchedRows(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows; delete from minipos_state_meta`); e != nil {
		t.Fatal(e)
	}
	first := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"same":{"id":"same"},"changed":{"id":"changed","version":1},"removed":{"id":"removed"}},"sequence":1}`)
	if e = p.Save(first); e != nil {
		t.Fatal(e)
	}
	var sameBefore, changedBefore time.Time
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='same'`).Scan(&sameBefore); e != nil {
		t.Fatal(e)
	}
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='changed'`).Scan(&changedBefore); e != nil {
		t.Fatal(e)
	}
	time.Sleep(10 * time.Millisecond)
	second := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"same":{"id":"same"},"changed":{"id":"changed","version":2},"added":{"id":"added"}},"sequence":2}`)
	if e = p.Save(second); e != nil {
		t.Fatal(e)
	}
	var sameAfter, changedAfter time.Time
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='same'`).Scan(&sameAfter); e != nil {
		t.Fatal(e)
	}
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='changed'`).Scan(&changedAfter); e != nil {
		t.Fatal(e)
	}
	if !sameAfter.Equal(sameBefore) {
		t.Fatal("unchanged row was rewritten")
	}
	if !changedAfter.After(changedBefore) {
		t.Fatal("changed row was not updated")
	}
	var count int
	if e = p.db.QueryRow(`select count(*) from minipos_state_rows where collection='configurations' and entity_key in ('removed','added')`).Scan(&count); e != nil || count != 1 {
		t.Fatalf("targeted add/delete failed: %d %v", count, e)
	}
}
func TestMiniPOSTypedProjectionIsAtomicAndDifferential(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from minipos_runtime_orders;delete from minipos_runtime_shifts;delete from minipos_runtime_operator_sessions;delete from minipos_runtime_identity_bindings;delete from minipos_runtime_employees;delete from minipos_runtime_products;delete from minipos_runtime_configurations`); e != nil {
		t.Fatal(e)
	}
	base := `{"products":{"product-typed":{"id":"product-typed","tenant_id":"org-a","sku":"SKU-A","barcode":"380000000001","name":"Coffee","unit":"pcs","price":{"amount":"%s","currency":"EUR"},"tax_group":"B","active":true,"status":"ACTIVE","version":%d,"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:01:00Z"}},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"org-a":{"id":"configuration-typed","tenant_id":"org-a","location_name":"Shop","location_address":"Sofia","workstation_name":"POS 1","fiscal_register_id":"FD1","version":1,"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:01:00Z"}},"sequence":1}`
	if e = p.Save([]byte(fmt.Sprintf(base, "2.50", 1))); e != nil {
		t.Fatal(e)
	}
	var tenant, amount, barcode string
	var version int64
	if e = p.db.QueryRow(`select organization_id,amount::text,version from minipos_runtime_products where id='product-typed'`).Scan(&tenant, &amount, &version); e != nil || tenant != "org-a" || amount != "2.50" || version != 1 {
		t.Fatal(tenant, amount, version, e)
	}
	if e = p.db.QueryRow(`select barcode from minipos_runtime_products where id='product-typed'`).Scan(&barcode); e != nil || barcode != "380000000001" {
		t.Fatal("barcode was not persisted", barcode, e)
	}
	raw, e := p.LoadTenantEntity("products", "org-a", "product-typed")
	if e != nil || !json.Valid(raw) || !bytes.Contains(raw, []byte(`380000000001`)) {
		t.Fatal("tenant typed read failed", string(raw), e)
	}
	if _, e = p.LoadTenantEntity("products", "org-b", "product-typed"); e == nil {
		t.Fatal("RLS typed read exposed foreign product")
	}
	configurationRows, e := p.LoadTenantEntities("configurations", "org-a")
	if e != nil || len(configurationRows) != 1 || !json.Valid(configurationRows[0]) {
		t.Fatal("tenant typed configuration read failed", len(configurationRows), e)
	}
	configurationRows, e = p.LoadTenantEntities("configurations", "org-b")
	if e != nil || len(configurationRows) != 0 {
		t.Fatal("RLS collection exposed foreign configuration", len(configurationRows), e)
	}
	rows, e := p.LoadTenantEntities("products", "org-a")
	if e != nil || len(rows) != 1 {
		t.Fatal("tenant collection read failed", len(rows), e)
	}
	rows, e = p.LoadTenantEntities("products", "org-b")
	if e != nil || len(rows) != 0 {
		t.Fatal("RLS collection exposed foreign product", len(rows), e)
	}
	reassigned := strings.Replace(fmt.Sprintf(base, "3.00", 2), "org-a", "org-b", 1)
	if e = p.Save([]byte(reassigned)); e == nil {
		t.Fatal("RLS-bound mutation reassigned an existing product to another organization")
	}
	if e = p.db.QueryRow(`select organization_id,version from minipos_runtime_products where id='product-typed'`).Scan(&tenant, &version); e != nil || tenant != "org-a" || version != 1 {
		t.Fatal("failed cross-organization mutation was not atomic", tenant, version, e)
	}
	if e = p.Save([]byte(fmt.Sprintf(base, "3.00", 2))); e != nil {
		t.Fatal(e)
	}
	if e = p.db.QueryRow(`select amount::text,version from minipos_runtime_products where id='product-typed'`).Scan(&amount, &version); e != nil || amount != "3.00" || version != 2 {
		t.Fatal(amount, version, e)
	}
	invalid := []byte(`{"products":{"product-typed":{"id":"product-typed","tenant_id":"org-a","sku":"SKU-A","name":"Coffee","unit":"pcs","price":{"amount":"4.00","currency":"BGN"},"tax_group":"B","active":true,"status":"ACTIVE","version":3,"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:02:00Z"}},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":2}`)
	if e = p.Save(invalid); e == nil {
		t.Fatal("non-EUR typed write accepted")
	}
	var persistedVersion int64
	if e = p.db.QueryRow(`select (payload->>'version')::bigint from minipos_state_rows where collection='products' and entity_key='product-typed'`).Scan(&persistedVersion); e != nil || persistedVersion != 2 {
		t.Fatal("compatibility row escaped rollback", persistedVersion, e)
	}
}

func TestMiniPOSTypedEmployeeSurvivesIdempotencyOnlySave(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from minipos_runtime_orders;delete from minipos_runtime_shifts;delete from minipos_runtime_operator_sessions;delete from minipos_runtime_identity_bindings;delete from minipos_runtime_employees;delete from minipos_runtime_products;delete from minipos_runtime_configurations;delete from minipos_runtime_api_replays;delete from minipos_runtime_webhook_inbox`); e != nil {
		t.Fatal(e)
	}
	first := []byte(`{"products":{},"employees":{"employee-1":{"id":"employee-1","tenant_id":"org-e2e","first_name":"Ada","last_name":"Lovelace","operator_code":"A001","roles":[],"active":true,"status":"ACTIVE","version":1,"created_at":"2026-08-08T10:00:00Z","updated_at":"2026-08-08T10:00:00Z"}},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{"org-e2e\nPATCH\n/public/v1/minipos/configuration\nconfig-key":{"hash":"a","status":200,"body":"e30=","content_type":"application/json"}},"webhook_inbox":{"event-1":{"event_id":"event-1","tenant_id":"org-e2e","hash":"c","raw":"e30=","received_at":"2026-08-08T10:00:00Z"}},"configurations":{"org-e2e":{"id":"configuration-1","tenant_id":"org-e2e","location_name":"E2E Shop","location_address":"Sofia","workstation_name":"POS 01","fiscal_register_id":"00000000-0000-4000-8000-000000000001","version":1,"created_at":"2026-08-08T10:00:00Z","updated_at":"2026-08-08T10:00:00Z"}},"sequence":2}`)
	if e = p.Save(first); e != nil {
		t.Fatal(e)
	}
	second := strings.Replace(string(first), `"api_replays":{`, `"api_replays":{"org-e2e\nPOST\n/public/v1/minipos/employees\nemployee-key":{"hash":"b","status":201,"body":"e30=","content_type":"application/json"},`, 1)
	if e = p.Save([]byte(second)); e != nil {
		t.Fatalf("idempotency-only save after typed employee: %v", e)
	}
	for _, table := range []string{"minipos_runtime_api_replays", "minipos_runtime_webhook_inbox"} {
		var count int
		if e = p.db.QueryRow(`select count(*) from ` + table + ` where organization_id='org-e2e'`).Scan(&count); e != nil || count < 1 {
			t.Fatalf("%s projection: count=%d err=%v", table, count, e)
		}
	}
	for collection, id := range map[string]string{
		"api_replays":   "org-e2e\nPATCH\n/public/v1/minipos/configuration\nconfig-key",
		"webhook_inbox": "event-1",
	} {
		raw, readErr := p.LoadTenantEntity(collection, "org-e2e", id)
		if readErr != nil || len(raw) == 0 {
			t.Fatalf("typed %s orchestration read failed: %s %v", collection, raw, readErr)
		}
		if _, readErr = p.LoadTenantEntity(collection, "org-other", id); !errors.Is(readErr, sql.ErrNoRows) {
			t.Fatalf("typed %s read exposed foreign organization: %v", collection, readErr)
		}
	}
	tx, e := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	_, _ = tx.Exec(`set local role beeminipos_tenant`)
	_, _ = tx.Exec(`select set_config('app.organization_id','org-other',true)`)
	var hidden int
	if e = tx.QueryRow(`select count(*) from minipos_runtime_webhook_inbox`).Scan(&hidden); e != nil || hidden != 0 {
		t.Fatal("RLS exposed foreign webhook inbox", hidden, e)
	}
}

func TestMiniPOSTypedCheckoutCheckpointIsAtomicAndTenantBound(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from minipos_runtime_checkout_results;delete from minipos_runtime_checkout_hashes`); e != nil {
		t.Fatal(e)
	}
	hash := strings.Repeat("a", 64)
	state := fmt.Sprintf(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{"org-checkout:key-1":{"id":"order-1","tenant_id":"org-checkout","state":"UNKNOWN","version":2}},"checkout_hashes":{"org-checkout:key-1":"%s"},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":1}`, hash)
	if e = p.Save([]byte(state)); e != nil {
		t.Fatal(e)
	}
	var resultTenant, hashTenant, persistedHash string
	if e = p.db.QueryRow(`select organization_id from minipos_runtime_checkout_results where replay_key='org-checkout:key-1'`).Scan(&resultTenant); e != nil || resultTenant != "org-checkout" {
		t.Fatal(resultTenant, e)
	}
	if e = p.db.QueryRow(`select organization_id,request_hash from minipos_runtime_checkout_hashes where replay_key='org-checkout:key-1'`).Scan(&hashTenant, &persistedHash); e != nil || hashTenant != "org-checkout" || persistedHash != hash {
		t.Fatal(hashTenant, persistedHash, e)
	}
	rawResult, readErr := p.LoadTenantEntity("checkouts", "org-checkout", "org-checkout:key-1")
	rawHash, hashErr := p.LoadTenantEntity("checkout_hashes", "org-checkout", "org-checkout:key-1")
	var typedHash string
	if readErr != nil || hashErr != nil || !strings.Contains(string(rawResult), `"id": "order-1"`) || json.Unmarshal(rawHash, &typedHash) != nil || typedHash != hash {
		t.Fatal("typed checkout checkpoint read failed", string(rawResult), string(rawHash), readErr, hashErr)
	}
	if _, readErr = p.LoadTenantEntity("checkouts", "org-other", "org-checkout:key-1"); !errors.Is(readErr, sql.ErrNoRows) {
		t.Fatal("typed checkout read exposed foreign organization", readErr)
	}
	tx, e := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	_, _ = tx.Exec(`set local role beeminipos_tenant`)
	_, _ = tx.Exec(`select set_config('app.organization_id','org-other',true)`)
	var hidden int
	if e = tx.QueryRow(`select count(*) from minipos_runtime_checkout_results`).Scan(&hidden); e != nil || hidden != 0 {
		t.Fatal("RLS exposed foreign checkout checkpoint", hidden, e)
	}
	_ = tx.Rollback()
	bad := strings.Replace(state, hash, "short", 1)
	if e = p.Save([]byte(bad)); e == nil {
		t.Fatal("invalid checkout request hash accepted")
	}
	if e = p.db.QueryRow(`select request_hash from minipos_runtime_checkout_hashes where replay_key='org-checkout:key-1'`).Scan(&persistedHash); e != nil || persistedHash != hash {
		t.Fatal("failed checkpoint write was not atomic", persistedHash, e)
	}
}

func TestMiniPOSStateRowsRejectUnknownData(t *testing.T) {
	if _, e := flattenSnapshot([]byte(`{"unknown":{}}`)); e == nil {
		t.Fatal("unknown collection accepted")
	}
	if _, e := rebuildSnapshot([]stateRow{{Collection: "sequence", Key: "wrong", Payload: []byte(`1`)}}); e == nil {
		t.Fatal("invalid singleton accepted")
	}
}

func TestMiniPOSPostgresRestartLegacyAndEmptyState(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from runtime_snapshots where aggregate='minipos'`); e != nil {
		t.Fatal(e)
	}
	raw := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"c":{"id":"c"}},"sequence":7}`)
	if e = p.Save(raw); e != nil {
		t.Fatal(e)
	}
	_ = p.Close()
	p, e = Open(url)
	if e != nil {
		t.Fatal(e)
	}
	loaded, e := p.Load()
	if e != nil {
		t.Fatal(e)
	}
	assertJSONEqual(t, raw, loaded)
	legacy := []byte(`{"products":{"legacy-product":{"id":"legacy-product","tenant_id":"org-migration","sku":"LEGACY-1","name":"Legacy product","unit":"pcs","price":{"amount":"1.00","currency":"EUR"},"tax_group":"B","active":true,"status":"ACTIVE","version":1,"created_at":"2026-08-08T10:00:00Z","updated_at":"2026-08-08T10:00:00Z"}},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":8}`)
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from minipos_runtime_products;delete from minipos_runtime_operator_sessions;delete from minipos_runtime_identity_bindings;delete from minipos_runtime_employees;delete from minipos_runtime_shifts;delete from minipos_runtime_orders;delete from minipos_runtime_configurations;delete from minipos_runtime_api_replays;delete from minipos_runtime_webhook_inbox;delete from minipos_runtime_checkout_results;delete from minipos_runtime_checkout_hashes`); e != nil {
		t.Fatal(e)
	}
	if _, e = p.db.Exec(`insert into runtime_snapshots(aggregate,payload,version,updated_at) values('minipos',$1::jsonb,1,now()) on conflict(aggregate) do update set payload=excluded.payload`, string(legacy)); e != nil {
		t.Fatal(e)
	}
	loaded, e = p.Load()
	if e != nil {
		t.Fatal(e)
	}
	assertJSONEqual(t, legacy, loaded)
	var tenant string
	if e = p.db.QueryRow(`select organization_id from minipos_runtime_products where id='legacy-product'`).Scan(&tenant); e != nil || tenant != "org-migration" {
		t.Fatalf("legacy state missing authoritative typed projection: tenant=%q err=%v", tenant, e)
	}
	empty := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":0}`)
	if e = p.Save(empty); e != nil {
		t.Fatal(e)
	}
	loaded, e = p.Load()
	if e != nil {
		t.Fatal(e)
	}
	assertJSONEqual(t, empty, loaded)
}
func TestMiniPOSVersionedSaveRejectsStaleInstance(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p1, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer p1.Close()
	p2, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer p2.Close()
	if _, e = p1.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from runtime_snapshots where aggregate='minipos'`); e != nil {
		t.Fatal(e)
	}
	_, generation1, e := p1.LoadVersioned()
	if e != nil || generation1 != 0 {
		t.Fatal(generation1, e)
	}
	_, generation2, e := p2.LoadVersioned()
	if e != nil || generation2 != 0 {
		t.Fatal(generation2, e)
	}
	first := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":1}`)
	if generation1, e = p1.SaveVersioned(first, generation1); e != nil || generation1 != 1 {
		t.Fatal(generation1, e)
	}
	stale := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":2}`)
	if _, e = p2.SaveVersioned(stale, generation2); !errors.Is(e, ErrConcurrentState) {
		t.Fatal("stale MiniPOS instance overwrote current state", e)
	}
	loaded, current, e := p2.LoadVersioned()
	if e != nil || current != 1 {
		t.Fatal(current, e)
	}
	assertJSONEqual(t, first, loaded)
}
func TestMiniPOSDeltaSaveTouchesOnlyExplicitRows(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from runtime_snapshots where aggregate='minipos'`); e != nil {
		t.Fatal(e)
	}
	baseline := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":1}`)
	generation, e := p.SaveVersioned(baseline, 0)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.db.Exec(`insert into minipos_state_rows(collection,entity_key,payload) values('orders','unmanaged','{"id":"unmanaged"}'::jsonb)`); e != nil {
		t.Fatal(e)
	}
	current := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":2}`)
	if _, e = p.SaveDeltaVersioned(baseline, current, generation); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = p.db.QueryRow(`select count(*) from minipos_state_rows where collection='orders' and entity_key='unmanaged'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("delta save scanned/deleted an unrelated MiniPOS row", count, e)
	}
}

func TestMiniPOSTypedOnlyRestartAndRollback(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from minipos_runtime_products;delete from minipos_runtime_operator_sessions;delete from minipos_runtime_identity_bindings;delete from minipos_runtime_employees;delete from minipos_runtime_shifts;delete from minipos_runtime_orders;delete from minipos_runtime_configurations;delete from minipos_runtime_api_replays;delete from minipos_runtime_webhook_inbox;delete from minipos_runtime_checkout_results;delete from minipos_runtime_checkout_hashes`); e != nil {
		t.Fatal(e)
	}
	empty := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":0}`)
	generation, e := p.SaveVersioned(empty, 0)
	if e != nil {
		t.Fatal(e)
	}
	v1 := []byte(`{"products":{"typed-only":{"id":"typed-only","tenant_id":"org-typed","sku":"T-1","name":"Typed","unit":"pcs","price":{"amount":"1.00","currency":"EUR"},"tax_group":"B","active":true,"status":"ACTIVE","version":1,"created_at":"2026-08-08T10:00:00Z","updated_at":"2026-08-08T10:00:00Z"}},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":1}`)
	generation, e = p.SaveDeltaVersioned(empty, v1, generation)
	if e != nil {
		t.Fatal(e)
	}
	var mode int
	if e = p.db.QueryRow(`select storage_mode from minipos_state_meta where singleton=true`).Scan(&mode); e != nil || mode != 2 {
		t.Fatal("typed-only mode was not activated", mode, e)
	}
	v2 := bytes.Replace(v1, []byte(`"version":1`), []byte(`"version":2`), 1)
	v2 = bytes.Replace(v2, []byte(`"sequence":1`), []byte(`"sequence":2`), 1)
	generation, e = p.SaveDeltaVersioned(v1, v2, generation)
	if e != nil {
		t.Fatal(e)
	}
	var compatibilityVersion, typedVersion int
	if e = p.db.QueryRow(`select (payload->>'version')::int from minipos_state_rows where collection='products' and entity_key='typed-only'`).Scan(&compatibilityVersion); e != nil || compatibilityVersion != 1 {
		t.Fatal("typed-only write still mutated compatibility state", compatibilityVersion, e)
	}
	if e = p.db.QueryRow(`select (payload->>'version')::int from minipos_runtime_products where id='typed-only'`).Scan(&typedVersion); e != nil || typedVersion != 2 {
		t.Fatal("typed-only projection not updated", typedVersion, e)
	}
	if _, e = p.db.Exec(`update minipos_state_rows set payload=jsonb_set(payload,'{version}','99'::jsonb) where collection='products' and entity_key='typed-only'`); e != nil {
		t.Fatal(e)
	}
	loaded, loadedGeneration, e := p.LoadVersioned()
	if e != nil || loadedGeneration != generation {
		t.Fatal(loadedGeneration, generation, e)
	}
	assertJSONEqual(t, v2, loaded)
	bad := bytes.Replace(v2, []byte(`"currency":"EUR"`), []byte(`"currency":"BGN"`), 1)
	if _, e = p.SaveDeltaVersioned(v2, bad, generation); e == nil {
		t.Fatal("typed-only constraint failure was accepted")
	}
	loaded, loadedGeneration, e = p.LoadVersioned()
	if e != nil || loadedGeneration != generation {
		t.Fatal("failed typed-only mutation changed generation", loadedGeneration, generation, e)
	}
	assertJSONEqual(t, v2, loaded)
}
func assertJSONEqual(t *testing.T, wantRaw, gotRaw []byte) {
	t.Helper()
	var want, got map[string]any
	_ = json.Unmarshal(wantRaw, &want)
	_ = json.Unmarshal(gotRaw, &got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("state mismatch\nwant=%#v\ngot=%#v", want, got)
	}
}
