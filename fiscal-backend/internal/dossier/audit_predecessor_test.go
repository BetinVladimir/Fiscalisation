package dossier

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fiscalisation/fiscal-backend/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Uses only the explicit disposable database created by the dossier runner.
func TestAuditPredecessorForwardMigration(t *testing.T) {
	dsn := os.Getenv("DOSSIER_TEST_ADMIN_URL")
	if dsn == "" {
		t.Skip("requires disposable Fiscal PostgreSQL runner")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	schema := fmt.Sprintf("audit_forward_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = conn.Exec(ctx, "create schema "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "set search_path=fiscal,public")
		_, _ = conn.Exec(ctx, "drop schema "+quoted+" cascade")
	}()
	if _, err = conn.Exec(ctx, "set search_path="+quoted+",public"); err != nil {
		t.Fatal(err)
	}
	// Clone the migrated physical table, then rehearse the actual old migration.
	if _, err = conn.Exec(ctx, `create table fiscal_runtime_audit (like fiscal.fiscal_runtime_audit including all);alter table fiscal_runtime_audit drop constraint if exists fiscal_audit_predecessor_consistent;alter table fiscal_runtime_audit drop column previous_hash`); err != nil {
		t.Fatal(err)
	}
	migrations := filepath.Join("..", "..", "..", "database", "fiscal")
	old, err := os.ReadFile(filepath.Join(migrations, "019_audit_immutability.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, string(old)); err != nil {
		t.Fatal(err)
	}
	repo := domain.NewMemoryRepository()
	for i := 0; i < 2; i++ {
		if err = repo.AppendAudit("audit-fixture", "operator", "OPERATOR_CHANGED", "operator", "operator-1", "", nil, map[string]any{"version": i + 1}); err != nil {
			t.Fatal(err)
		}
	}
	events := repo.AuditEvents("audit-fixture")
	if len(events) != 2 || events[1].PrevHash == "" {
		t.Fatal("domain chain fixture missing")
	}
	canonical, _ := json.Marshal(events[1])
	legacyMap := map[string]any{}
	_ = json.Unmarshal(canonical, &legacyMap)
	delete(legacyMap, "previous_hash")
	legacyMap["prev_hash"] = events[1].PrevHash
	legacy, _ := json.Marshal(legacyMap)
	insert := `insert into fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload) values($1,'audit-fixture','operator','OPERATOR_CHANGED','operator','operator-1',now(),$2,$3::jsonb)`
	for i, raw := range [][]byte{canonical, legacy} {
		if _, err = conn.Exec(ctx, insert, fmt.Sprintf("event-%d", i), events[1].EventHash, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	var beforeJSON, beforeHash, predecessor string
	if err = conn.QueryRow(ctx, `select payload::text,event_hash,previous_hash from fiscal_runtime_audit where event_id='event-0'`).Scan(&beforeJSON, &beforeHash, &predecessor); err != nil {
		t.Fatal(err)
	}
	if predecessor != "" {
		t.Fatal("old migration did not reproduce the actual canonical-key bug")
	}
	forward, err := os.ReadFile(filepath.Join(migrations, "021_audit_predecessor_compatibility.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, string(forward)); err != nil {
		t.Fatal(err)
	}
	var afterJSON, afterHash string
	if err = conn.QueryRow(ctx, `select payload::text,event_hash,previous_hash from fiscal_runtime_audit where event_id='event-0'`).Scan(&afterJSON, &afterHash, &predecessor); err != nil {
		t.Fatal(err)
	}
	if beforeJSON != afterJSON || beforeHash != afterHash || predecessor != events[1].PrevHash {
		t.Fatal("forward migration changed source evidence or lost canonical predecessor")
	}
	if err = conn.QueryRow(ctx, `select previous_hash from fiscal_runtime_audit where event_id='event-1'`).Scan(&predecessor); err != nil || predecessor != events[1].PrevHash {
		t.Fatalf("legacy predecessor lost: %v", err)
	}
	for _, sql := range []string{`update fiscal_runtime_audit set event_hash='changed' where event_id='event-0'`, `delete from fiscal_runtime_audit where event_id='event-0'`} {
		if _, err = conn.Exec(ctx, sql); err == nil || !strings.Contains(err.Error(), "FISCAL_AUDIT_IS_IMMUTABLE") {
			t.Fatalf("append-only protection lost: %v", err)
		}
	}
	conflict := map[string]any{"previous_hash": events[1].PrevHash, "prev_hash": strings.Repeat("f", 64)}
	raw, _ := json.Marshal(conflict)
	if _, err = conn.Exec(ctx, insert, "conflicting", events[1].EventHash, string(raw)); err == nil {
		t.Fatal("ambiguous predecessor accepted")
	}
	// Future typed projection must read both formats; historical typed rows are not rewritten.
	var definition string
	if err = conn.QueryRow(ctx, `select pg_get_functiondef('project_runtime_audit_to_supto_events()'::regprocedure)`).Scan(&definition); err != nil || !strings.Contains(definition, "previous_hash") {
		t.Fatalf("projection compatibility missing: %v", err)
	}
}

func TestAuditPredecessorMigrationRefusesHistoricalConflict(t *testing.T) {
	dsn := os.Getenv("DOSSIER_TEST_ADMIN_URL")
	if dsn == "" {
		t.Skip("requires disposable Fiscal PostgreSQL runner")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	schema := fmt.Sprintf("audit_conflict_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = conn.Exec(ctx, "create schema "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "rollback")
		_, _ = conn.Exec(ctx, "set search_path=fiscal,public")
		_, _ = conn.Exec(ctx, "drop schema "+quoted+" cascade")
	}()
	if _, err = conn.Exec(ctx, "set search_path="+quoted+",public"); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `create table fiscal_runtime_audit (like fiscal.fiscal_runtime_audit including all);alter table fiscal_runtime_audit drop constraint if exists fiscal_audit_predecessor_consistent;alter table fiscal_runtime_audit drop column previous_hash`); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile("../../../database/fiscal/019_audit_immutability.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, string(old)); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `insert into fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload) values('conflict','fixture','actor','ACTION','operator','one',now(),repeat('c',64),jsonb_build_object('previous_hash',repeat('a',64),'prev_hash',repeat('b',64)))`); err != nil {
		t.Fatal(err)
	}
	var before string
	if err = conn.QueryRow(ctx, `select payload::text from fiscal_runtime_audit where event_id='conflict'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	forward, err := os.ReadFile("../../../database/fiscal/021_audit_predecessor_compatibility.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, string(forward)); err == nil || !strings.Contains(err.Error(), "FISCAL_AUDIT_PREDECESSOR_CONFLICT") {
		t.Fatalf("historical ambiguity did not block migration: %v", err)
	}
	if _, err = conn.Exec(ctx, "rollback"); err != nil {
		t.Fatal(err)
	}
	var after, previous string
	if err = conn.QueryRow(ctx, `select payload::text,previous_hash from fiscal_runtime_audit where event_id='conflict'`).Scan(&after, &previous); err != nil {
		t.Fatal(err)
	}
	if after != before || previous != strings.Repeat("b", 64) {
		t.Fatal("failed migration changed source payload or legacy generated metadata")
	}
	var count int
	if err = conn.QueryRow(ctx, `select count(*) from pg_constraint where conrelid='fiscal_runtime_audit'::regclass and conname='fiscal_audit_predecessor_consistent'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed migration left partial constraint: %v", err)
	}
	if _, err = conn.Exec(ctx, `delete from fiscal_runtime_audit where event_id='conflict'`); err == nil || !strings.Contains(err.Error(), "FISCAL_AUDIT_IS_IMMUTABLE") {
		t.Fatalf("failed migration lost append-only protection: %v", err)
	}
}
