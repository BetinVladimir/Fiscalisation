package dossier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniPOSPostgresSnapshot(t *testing.T) {
	adminURL, readerURL := os.Getenv("DOSSIER_MINIPOS_TEST_ADMIN_URL"), os.Getenv("DOSSIER_MINIPOS_TEST_READER_URL")
	if adminURL == "" || readerURL == "" {
		t.Skip("actual disposable MiniPOS PostgreSQL required; use scripts/dossier-minipos-postgres-integration.sh")
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
	// All credentials and records are disposable test fixtures, never acquired production data.
	_, err = admin.Exec(ctx, `insert into public.minipos_runtime_orders(id,organization_id,external_id,shift_id,register_id,operator_code,state,total,currency,lines,version,created_at,updated_at,payload,payments,fiscal_sale_id,fiscal_operation_id,fiscal_version) values
 ('mini-a','org-a','external-a','shift-a','register-a','A001','UNKNOWN',9007199254740993.17,'EUR','[{"line_id":"line-a","name":"Български","quantity":"1.000","unit_price":{"amount":"9007199254740993.17","currency":"EUR","credential":"NESTED_SECRET"},"tax_group":"A","credential":"LINE_SECRET"}]',9007199254740993,now(),now(),'{"credential":"PAYLOAD_SECRET","receipt_session_id":"not-projected"}','[{"payment_id":"pay-a","type":"CARD","amount":{"amount":"9007199254740993.17","currency":"EUR","credential":"PAYMENT_MONEY_SECRET"},"terminal_policy":"REQUIRED","credential":"PAYMENT_SECRET"}]','fiscal-sale-a','client-operation-a',9223372036854775807),
 ('mini-b','org-b','external-b','shift-b','register-b','B001','UNKNOWN',1.01,'EUR','[]',1,now(),now(),'{"foreign":"FOREIGN_ORG_SECRET"}','[]',null,null,null)
 on conflict(id) do update set state=excluded.state,total=excluded.total,version=excluded.version,lines=excluded.lines,payments=excluded.payments,payload=excluded.payload`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(ctx, `insert into public.minipos_runtime_checkout_results(replay_key,organization_id,order_id,state,version,fiscal_sale_id,fiscal_operation_id,payload) values('checkout-a','org-a','mini-a','UNKNOWN',9007199254740993,'fiscal-sale-a','client-operation-a','{"credential":"CHECKOUT_PAYLOAD_SECRET"}') on conflict(replay_key) do update set state=excluded.state`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(ctx, `insert into public.minipos_runtime_products(id,organization_id,sku,name,unit,amount,currency,tax_group,active,status,version,created_at,updated_at,payload) values('product-a','org-a','sku-a','Товар','pcs',9007199254740993.17,'EUR','A',true,'ACTIVE',1,now(),now(),'{"credential":"PRODUCT_PAYLOAD_SECRET"}') on conflict do nothing;
 insert into public.minipos_runtime_tax_groups(id,organization_id,code,name,rate,status,version,created_at,updated_at,payload) values('tax-a','org-a','A','Tax',20.25,'ACTIVE',1,now(),now(),'{"credential":"TAX_PAYLOAD_SECRET"}') on conflict do nothing;
 insert into public.minipos_runtime_employees(id,organization_id,first_name,last_name,operator_code,roles,active,status,version,created_at,updated_at,payload) values('employee-a','org-a','Иван','Иванов','A001','["ROLE_SECRET"]',true,'ACTIVE',1,now(),now(),'{"credential":"EMPLOYEE_PAYLOAD_SECRET"}') on conflict do nothing;
 insert into public.minipos_runtime_shifts(id,organization_id,register_id,employee_id,state,version,opened_at,created_at,updated_at,close_error,payload) values('shift-a','org-a','register-a','employee-a','OPEN',1,now(),now(),now(),'ERROR_SECRET','{"credential":"SHIFT_PAYLOAD_SECRET"}') on conflict do nothing;
 insert into public.minipos_runtime_checkout_hashes(replay_key,organization_id,request_hash) values('checkout-a','org-a',repeat('a',64)) on conflict do nothing;
 insert into public.minipos_runtime_api_replays(replay_key,organization_id,method,path,request_hash,status,payload) values('private-replay','org-a','POST','/private',repeat('b',64),200,'{"credential":"API_REPLAY_SECRET"}') on conflict do nothing`)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := reader.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	contract, err := InspectProfile(ctx, tx, "public", "minipos")
	tx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reviewedRaw, err := os.ReadFile("../../../docs/remediation/2026-10-17/25_MINIPOS_PG_DICTIONARY.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	var reviewed Contract
	if err = json.Unmarshal(reviewedRaw, &reviewed); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(contract)
	wantJSON, _ := json.Marshal(reviewed)
	if string(gotJSON) != string(wantJSON) {
		t.Fatal("committed MiniPOS dictionary drift")
	}
	options := func(out string) Options {
		return Options{Profile: "minipos", Schema: "public", Tenant: "org-a", Output: out, SourceCommit: "synthetic-minipos-fixture", Contract: contract}
	}
	readRows := func(t *testing.T, out, table string) []map[string]*string {
		t.Helper()
		raw, e := os.ReadFile(filepath.Join(out, table+".jsonl"))
		if e != nil {
			t.Fatal(e)
		}
		var result []map[string]*string
		var columns []string
		for _, item := range contract.Tables {
			if item.Name == table {
				columns = item.ExportColumns
			}
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
			if line == "" {
				continue
			}
			var cells []*string
			if e = json.Unmarshal([]byte(line), &cells); e != nil {
				t.Fatal(e)
			}
			row := map[string]*string{}
			for i, col := range columns {
				row[col] = cells[i]
			}
			result = append(result, row)
		}
		return result
	}
	t.Run("consistent_snapshot_precision_secret_projection_and_isolation", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "snapshot")
		err := export(ctx, reader, options(out), func() error {
			_, e := admin.Exec(ctx, `update public.minipos_runtime_orders set state='FISCALIZED',version=version+1 where id='mini-a';update public.minipos_runtime_checkout_results set state='FISCALIZED' where replay_key='checkout-a'`)
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if err = Verify(out); err != nil {
			t.Fatal(err)
		}
		rows := readRows(t, out, "minipos_runtime_orders")
		if len(rows) != 1 {
			t.Fatal("tenant leak", rows)
		}
		row := rows[0]
		if *row["state"] != "UNKNOWN" || *row["version"] != "9007199254740993" || *row["total"] != "9007199254740993.17" || *row["fiscal_version"] != "9223372036854775807" {
			t.Fatal("snapshot/precision loss")
		}
		if !strings.Contains(*row["lines"], "9007199254740993.17") || !strings.Contains(*row["lines"], "Български") {
			t.Fatal("line evidence lost")
		}
		checkout := readRows(t, out, "minipos_runtime_checkout_results")
		if *checkout[0]["state"] != "UNKNOWN" {
			t.Fatal("cross-table snapshot mismatch")
		}
		files, _ := os.ReadDir(out)
		for _, file := range files {
			body, _ := os.ReadFile(filepath.Join(out, file.Name()))
			if strings.Contains(string(body), "_SECRET") || strings.Contains(string(body), "org-b") {
				t.Fatal("secret/foreign tenant leaked", file.Name())
			}
		}
		other := options(filepath.Join(t.TempDir(), "org-b"))
		other.Tenant = "org-b"
		if err = Export(ctx, reader, other); err != nil {
			t.Fatal(err)
		}
		foreign := readRows(t, other.Output, "minipos_runtime_orders")
		if len(foreign) != 1 || *foreign[0]["id"] != "mini-b" {
			t.Fatal("organization isolation")
		}
		// Reusing a MiniPOS contract with the default Fiscal profile must fail closed.
		wrong := options(filepath.Join(t.TempDir(), "wrong-profile"))
		wrong.Profile = "fiscal"
		if err = Export(ctx, reader, wrong); err == nil {
			t.Fatal("cross-store profile accepted")
		}
		path := filepath.Join(out, "minipos_runtime_orders.jsonl")
		os.WriteFile(path, []byte("[]\n"), 0600)
		if err = Verify(out); err == nil {
			t.Fatal("archive tamper accepted")
		}
	})
	t.Run("ordinary_export_no_mutation", func(t *testing.T) {
		query := `select md5(string_agg(row_to_json(t)::text,'' order by id)) from public.minipos_runtime_orders t`
		var before, after string
		admin.QueryRow(ctx, query).Scan(&before)
		if err := Export(ctx, reader, options(filepath.Join(t.TempDir(), "unchanged"))); err != nil {
			t.Fatal(err)
		}
		admin.QueryRow(ctx, query).Scan(&after)
		if before != after {
			t.Fatal("source mutation")
		}
	})
	t.Run("schema_and_rls_drift", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `alter table public.minipos_runtime_orders add column dossier_drift text`); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "drift")
		if err := Export(ctx, reader, options(out)); err == nil {
			t.Fatal("schema drift accepted")
		}
		admin.Exec(ctx, `alter table public.minipos_runtime_orders drop column dossier_drift`)
		if _, err := admin.Exec(ctx, `alter table public.minipos_runtime_orders disable row level security`); err != nil {
			t.Fatal(err)
		}
		defer admin.Exec(ctx, `alter table public.minipos_runtime_orders enable row level security`)
		if err := Export(ctx, reader, options(filepath.Join(t.TempDir(), "rls"))); err == nil {
			t.Fatal("RLS drift accepted")
		}
	})
	t.Run("owner_no_clobber_readonly", func(t *testing.T) {
		if err := Export(ctx, admin, options(filepath.Join(t.TempDir(), "owner"))); err == nil {
			t.Fatal("superuser accepted")
		}
		out := t.TempDir()
		os.WriteFile(filepath.Join(out, "sentinel"), []byte("keep"), 0600)
		if err := Export(ctx, reader, options(out)); err == nil {
			t.Fatal("clobber accepted")
		}
		body, _ := os.ReadFile(filepath.Join(out, "sentinel"))
		if string(body) != "keep" {
			t.Fatal("output overwritten")
		}
		err := export(ctx, reader, options(filepath.Join(t.TempDir(), "readonly")), func() error {
			_, e := reader.Exec(ctx, `update public.minipos_runtime_orders set version=1 where id='mini-a'`)
			if e == nil {
				t.Fatal("write allowed")
			}
			return e
		})
		if err == nil {
			t.Fatal("readonly failure swallowed")
		}
	})
	t.Run("malformed_known_money_refused", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `update public.minipos_runtime_orders set payments='[{"type":"CARD","amount":{"amount":{"secret":"MALFORMED_SECRET"},"currency":"EUR"}}]' where id='mini-a'`); err != nil {
			t.Fatal(err)
		}
		defer admin.Exec(ctx, `update public.minipos_runtime_orders set payments='[]' where id='mini-a'`)
		out := filepath.Join(t.TempDir(), "bad-money")
		if err := Export(ctx, reader, options(out)); err == nil {
			t.Fatal("malformed known field accepted")
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("partial output")
		}
	})
}

func TestMiniPOSMigrationSourceInventory(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/remediation/2026-10-17/26_MINIPOS_PG_SCHEMA_SOURCES.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		SHA256 map[string]string `json:"sha256"`
	}
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("../../../database/minipos")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(m.SHA256) {
		t.Fatal("MiniPOS migration inventory drift")
	}
	for name, want := range m.SHA256 {
		body, e := os.ReadFile(filepath.Join("../../..", name))
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("MiniPOS source drift", name)
		}
	}
}
