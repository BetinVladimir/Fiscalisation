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

func TestPostgresSnapshot(t *testing.T) {
	adminURL := os.Getenv("DOSSIER_TEST_ADMIN_URL")
	readerURL := os.Getenv("DOSSIER_TEST_READER_URL")
	if adminURL == "" || readerURL == "" {
		t.Skip("requires disposable migrated PostgreSQL; see scripts/dossier-postgres-integration.sh")
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
	if _, err := admin.Exec(ctx, `grant select,insert,update,delete on fiscal.fiscal_runtime_operations to beefiscal_tenant`); err != nil {
		t.Fatal(err)
	}
	// Fixtures affect only explicitly supplied disposable DB. No production credentials are used.
	_, err = admin.Exec(ctx, `insert into fiscal.fiscal_runtime_operations(id,tenant_id,type,state,version,simulated,created_at,updated_at,payload) values('dossier-a','tenant-a','SALE_FINALIZE','UNKNOWN',9007199254740993,false,now(),now(),'{"unicode":"Български","big":9223372036854775807,"zero":"\\u0000 escaped"}'),('dossier-b','tenant-b','SALE_FINALIZE','FISCALIZED',1,false,now(),now(),'{"foreign":"TENANT_B_SECRET"}') on conflict(id) do update set state=excluded.state,version=excluded.version,payload=excluded.payload`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(ctx, `insert into fiscal.fiscal_runtime_artifacts(tenant_id,id,body,sha256) values('tenant-a','binary',decode('00ff807f','hex'),repeat('a',64)) on conflict do nothing`)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := reader.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Inspect(ctx, tx, "fiscal")
	tx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dictionaryRaw, err := os.ReadFile("../../../docs/remediation/2026-10-17/22_FISCAL_PG_DICTIONARY.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	var reviewed Contract
	if err = json.Unmarshal(dictionaryRaw, &reviewed); err != nil {
		t.Fatal(err)
	}
	actualJSON, _ := json.Marshal(contract)
	reviewedJSON, _ := json.Marshal(reviewed)
	if string(actualJSON) != string(reviewedJSON) {
		t.Fatal("committed schema dictionary drift")
	}
	options := func(out string) Options {
		return Options{Schema: "fiscal", Tenant: "tenant-a", Output: out, SourceCommit: "synthetic-integration-fixture", Contract: contract}
	}
	t.Run("snapshot_tenant_bigint_blob_no_mutation", func(t *testing.T) {
		before := []byte{}
		if err := admin.QueryRow(ctx, `select row_to_json(o)::text from fiscal.fiscal_runtime_operations o where id='dossier-a'`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "export")
		err := export(ctx, reader, options(out), func() error {
			_, e := admin.Exec(ctx, `update fiscal.fiscal_runtime_operations set state='FISCALIZED',version=version+1 where id='dossier-a'`)
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(out); err != nil {
			t.Fatal(err)
		}
		rows, err := os.ReadFile(filepath.Join(out, "fiscal_runtime_operations.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(rows), "TENANT_B_SECRET") || strings.Contains(string(rows), "tenant-b") {
			t.Fatal("cross-tenant disclosure")
		}
		var row []*string
		if err = json.Unmarshal(rows, &row); err != nil {
			t.Fatal(err)
		}
		got := map[string]*string{}
		for _, table := range contract.Tables {
			if table.Name == "fiscal_runtime_operations" {
				for i, c := range table.Columns {
					got[c.Name] = row[i]
				}
			}
		}
		if *got["state"] != "UNKNOWN" || *got["version"] != "9007199254740993" || !strings.Contains(*got["payload"], "9223372036854775807") {
			t.Fatal("lost snapshot/precision", got)
		}
		blob, _ := os.ReadFile(filepath.Join(out, "fiscal_runtime_artifacts.jsonl"))
		if !strings.Contains(string(blob), `\\x00ff807f`) {
			t.Fatal("bytea loss", string(blob))
		}
		var after string
		admin.QueryRow(ctx, `select state from fiscal.fiscal_runtime_operations where id='dossier-a'`).Scan(&after)
		if after != "FISCALIZED" {
			t.Fatal("writer update lost")
		}
		if len(before) == 0 {
			t.Fatal("no initial fixture")
		}
		// Tampered archive cannot retain a valid integrity result.
		path := filepath.Join(out, "fiscal_runtime_operations.jsonl")
		original, _ := os.ReadFile(path)
		os.WriteFile(path, []byte("[]\n"), 0600)
		if err := Verify(out); err == nil {
			t.Fatal("tamper accepted")
		}
		os.WriteFile(path, original, 0600)
		// Second tenant sees only its own operation.
		second := options(filepath.Join(t.TempDir(), "second"))
		second.Tenant = "tenant-b"
		if err = Export(ctx, reader, second); err != nil {
			t.Fatal(err)
		}
		other, _ := os.ReadFile(filepath.Join(second.Output, "fiscal_runtime_operations.jsonl"))
		if strings.Contains(string(other), "dossier-a") || !strings.Contains(string(other), "dossier-b") {
			t.Fatal("tenant isolation")
		}
	})
	t.Run("owner_rejected", func(t *testing.T) {
		if err := Export(ctx, admin, options(filepath.Join(t.TempDir(), "owner"))); err == nil {
			t.Fatal("superuser accepted")
		}
	})
	t.Run("non_superuser_owner_rejected", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `grant create on schema fiscal to beefiscal_tenant; alter table fiscal.fiscal_runtime_operations owner to beefiscal_tenant`); err != nil {
			t.Fatal(err)
		}
		defer admin.Exec(ctx, `alter table fiscal.fiscal_runtime_operations owner to postgres; grant select,insert,update,delete on fiscal.fiscal_runtime_operations to beefiscal_tenant; revoke create on schema fiscal from beefiscal_tenant`)
		if err := Export(ctx, reader, options(filepath.Join(t.TempDir(), "owner"))); err == nil {
			t.Fatal("non-super table owner accepted")
		}
	})
	t.Run("disabled_rls_rejected", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `alter table fiscal.fiscal_runtime_operations disable row level security`); err != nil {
			t.Fatal(err)
		}
		defer admin.Exec(ctx, `alter table fiscal.fiscal_runtime_operations enable row level security`)
		if err := Export(ctx, reader, options(filepath.Join(t.TempDir(), "rls"))); err == nil {
			t.Fatal("disabled RLS accepted")
		}
	})
	t.Run("no_clobber", func(t *testing.T) {
		out := t.TempDir()
		sentinel := filepath.Join(out, "sentinel")
		os.WriteFile(sentinel, []byte("keep"), 0600)
		if err := Export(ctx, reader, options(out)); err == nil {
			t.Fatal("existing output accepted")
		}
		b, _ := os.ReadFile(sentinel)
		if string(b) != "keep" {
			t.Fatal("overwritten")
		}
	})
	t.Run("schema_drift", func(t *testing.T) {
		_, err := admin.Exec(ctx, `alter table fiscal.fiscal_runtime_operations add column dossier_drift text`)
		if err != nil {
			t.Fatal(err)
		}
		defer admin.Exec(ctx, `alter table fiscal.fiscal_runtime_operations drop column dossier_drift`)
		out := filepath.Join(t.TempDir(), "drift")
		if err = Export(ctx, reader, options(out)); err == nil {
			t.Fatal("drift accepted")
		}
		if _, e := os.Stat(out); !os.IsNotExist(e) {
			t.Fatal("partial output")
		}
	})
	t.Run("read_only_transaction", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "ro")
		err := export(ctx, reader, options(out), func() error {
			_, e := reader.Exec(ctx, `update fiscal.fiscal_runtime_operations set version=2 where id='dossier-a'`)
			if e == nil {
				t.Fatal("write allowed")
			}
			return e
		})
		if err == nil {
			t.Fatal("aborted readonly tx should fail")
		}
		if _, e := os.Stat(out); !os.IsNotExist(e) {
			t.Fatal("partial readonly output")
		}
	})
	t.Run("ordinary_export_no_changes", func(t *testing.T) {
		var before, after string
		query := `select md5(string_agg(row_to_json(t)::text,'' order by id)) from fiscal.fiscal_runtime_operations t`
		admin.QueryRow(ctx, query).Scan(&before)
		if err := Export(ctx, reader, options(filepath.Join(t.TempDir(), "unchanged"))); err != nil {
			t.Fatal(err)
		}
		admin.QueryRow(ctx, query).Scan(&after)
		if before != after {
			t.Fatal("export modified source")
		}
	})
}

func TestMigrationSourceInventory(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/remediation/2026-10-17/23_FISCAL_PG_SCHEMA_SOURCES.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SHA256 map[string]string `json:"sha256"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("../../../database/fiscal")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(manifest.SHA256) {
		t.Fatal("migration inventory drift")
	}
	for name, want := range manifest.SHA256 {
		body, e := os.ReadFile(filepath.Join("../../..", name))
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("migration source drift:", name)
		}
	}
}

func TestFixturePreparationSourceInventory(t *testing.T) {
	for _, name := range []string{"23_FISCAL_PG_SCHEMA_SOURCES.generated.json", "26_MINIPOS_PG_SCHEMA_SOURCES.generated.json"} {
		raw, err := os.ReadFile("../../../docs/remediation/2026-10-17/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Preparer struct {
				Path   string `json:"path"`
				SHA256 string `json:"sha256"`
			} `json:"fixture_preparer"`
		}
		if err = json.Unmarshal(raw, &manifest); err != nil {
			t.Fatal(err)
		}
		if manifest.Preparer.Path != "scripts/prepare-fiscal-pg-fixture.py" {
			t.Fatal("fixture preparer identity drift", name)
		}
		source, err := os.ReadFile("../../../" + manifest.Preparer.Path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(source)
		if hex.EncodeToString(sum[:]) != manifest.Preparer.SHA256 {
			t.Fatal("fixture preparer source drift", name)
		}
	}
}
