package persistence

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func newTestUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

func TestUNPUniqueConstraint(t *testing.T) {
	db := mvpNewSchemaFixture(t, 22)
	ctx := context.Background()

	// Verify required schema objects exist.
	for _, table := range []string{
		"fiscal.fiscal_operations",
		"fiscal.client_operation",
		"fiscal.receipt_session",
		"fiscal.unp_allocations",
		"fiscal.fiscal_runtime_replays",
		"fiscal.fiscal_runtime_outbox",
	} {
		var exists bool
		if err := db.QueryRowContext(ctx,
			"SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			t.Fatalf("to_regclass(%s): %v", table, err)
		}
		if !exists {
			t.Fatalf("required table missing: %s", table)
		}
	}

	const tenant = "10000000-0000-4000-8000-000000000001"
	alloc1 := newTestUUID(t)
	alloc2 := newTestUUID(t)

	// Insert first valid allocation (sequence 1).
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `SET LOCAL app.tenant_id = '`+tenant+`'`); err != nil {
		tx.Rollback()
		t.Fatal("set app.tenant_id:", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO fiscal.unp_allocations
			(tenant_id, allocation_id, lease_id, fiscal_device_number, operator_code, unp_sequence, unp, status)
		VALUES ($1, $2, NULL, 'AB123456', 'A001', 1, 'AB123456-A001-0000001', 'ALLOCATED')`,
		tenant, alloc1)
	if err != nil {
		tx.Rollback()
		t.Fatal("insert first allocation:", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal("commit first allocation:", err)
	}

	// Insert duplicate UNP (different allocation_id, sequence 2) — must get SQLSTATE 23505.
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx2.ExecContext(ctx, `SET LOCAL app.tenant_id = '`+tenant+`'`); err != nil {
		tx2.Rollback()
		t.Fatal("set app.tenant_id:", err)
	}
	_, dupErr := tx2.ExecContext(ctx, `
		INSERT INTO fiscal.unp_allocations
			(tenant_id, allocation_id, lease_id, fiscal_device_number, operator_code, unp_sequence, unp, status)
		VALUES ($1, $2, NULL, 'AB123456', 'A001', 2, 'AB123456-A001-0000001', 'ALLOCATED')`,
		tenant, alloc2)
	tx2.Rollback()
	if dupErr == nil {
		t.Fatal("expected duplicate UNP to fail with SQLSTATE 23505, but it succeeded")
	}
	var pgErr *pgconn.PgError
	if !errors.As(dupErr, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("expected SQLSTATE 23505, got: %v", dupErr)
	}

	// Cross-tenant control: different tenant with same client_operation_id must succeed.
	// (unp_allocations has no client_operation_id column; we verify a different tenant
	//  can insert the same unp value, since UNIQUE is (tenant_id, unp) not global.)
	const tenant2 = "20000000-0000-4000-8000-000000000002"
	alloc3 := newTestUUID(t)
	tx3, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx3.ExecContext(ctx, `SET LOCAL app.tenant_id = '`+tenant2+`'`); err != nil {
		tx3.Rollback()
		t.Fatal("set app.tenant_id tenant2:", err)
	}
	_, err = tx3.ExecContext(ctx, `
		INSERT INTO fiscal.unp_allocations
			(tenant_id, allocation_id, lease_id, fiscal_device_number, operator_code, unp_sequence, unp, status)
		VALUES ($1, $2, NULL, 'AB123456', 'A001', 1, 'AB123456-A001-0000001', 'ALLOCATED')`,
		tenant2, alloc3)
	if err != nil {
		tx3.Rollback()
		t.Fatalf("cross-tenant insert (same unp, different tenant) must succeed: %v", err)
	}
	if err = tx3.Commit(); err != nil {
		t.Fatal("commit cross-tenant allocation:", err)
	}
}
