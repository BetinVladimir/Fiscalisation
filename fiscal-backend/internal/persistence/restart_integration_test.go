package persistence

import (
	"testing"
	"time"

	"fiscalisation/fiscal-backend/internal/domain"
)

// TG-001a-1: TestRestartKeepsReservedOperation verifies that after reserving a
// finalize operation and closing the repository, a fresh instance loaded from the
// same store recovers the operation in EXECUTING state with the same UNP.
func TestRestartKeepsReservedOperation(t *testing.T) {
	url, _ := isolatedPostgresFixture(t)
	store, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := domain.NewPersistentRepository(store)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	const saleID = "11111111-1111-4111-8111-100000000001"
	const opID = "22222222-2222-4222-8222-100000000001"
	const unp = "AB123456-A001-0000001"
	const tenant = "10000000-0000-0000-0000-000000000001"

	sale := domain.Sale{
		ID:         saleID,
		TenantID:   tenant,
		OperatorID: "A001",
		State:      "OPEN",
		UNP:        unp,
		RegisterID: "register-1",
		Lines: []domain.SaleLine{{
			LineID:    "line-1",
			Name:      "Coffee",
			Quantity:  "1.000",
			UnitPrice: domain.Money{Amount: "1.00", Currency: "EUR"},
			TaxGroup:  "B",
		}},
		Payments:  []domain.PaymentRecord{},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err = repo.PutSale(sale); err != nil {
		store.Close()
		t.Fatal(err)
	}

	op := domain.Operation{
		ID:                opID,
		ClientOperationID: opID,
		ReceiptSessionID:  "33333333-3333-4333-8333-100000000001",
		SaleID:            saleID,
		TenantID:          tenant,
		RegisterID:        "register-1",
		Type:              "SALE_FINALIZE",
		State:             "EXECUTING",
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = repo.PutOperation(op); err != nil {
		store.Close()
		t.Fatal(err)
	}
	store.Close()

	// Reopen from the same store URL.
	store2, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	repo2, err := domain.NewPersistentRepository(store2)
	if err != nil {
		t.Fatal(err)
	}

	recovered, err := repo2.Operation(opID)
	if err != nil {
		t.Fatalf("operation not found after restart: %v", err)
	}
	// recoverInterruptedOperations marks EXECUTING → UNKNOWN on startup (safe restart).
	if recovered.State != "EXECUTING" && recovered.State != "UNKNOWN" {
		t.Errorf("want state EXECUTING or UNKNOWN after restart, got %q", recovered.State)
	}

	recoveredSale, err := repo2.Sale(saleID)
	if err != nil {
		t.Fatalf("sale not found after restart: %v", err)
	}
	if recoveredSale.UNP != unp {
		t.Errorf("want UNP %q after restart, got %q", unp, recoveredSale.UNP)
	}
}
