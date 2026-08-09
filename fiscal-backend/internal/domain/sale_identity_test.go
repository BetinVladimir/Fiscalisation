package domain

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestPutSaleEnforcesTenantScopedExternalIDAndUNP(t *testing.T) {
	r := NewMemoryRepository()
	now := time.Now().UTC()
	base := Sale{ID: "sale-1", TenantID: "tenant-a", ExternalID: "order-1", RegisterID: "register-1", OperatorID: "A001", UNP: "register-1-A001-0000001", State: "OPEN", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, CreatedAt: now, UpdatedAt: now}
	if err := r.PutSale(base); err != nil {
		t.Fatal(err)
	}
	duplicateExternal := base
	duplicateExternal.ID = "sale-2"
	duplicateExternal.UNP = "register-1-A001-0000002"
	if err := r.PutSale(duplicateExternal); !errors.Is(err, ErrSaleExternalIDConflict) {
		t.Fatalf("duplicate external_id accepted: %v", err)
	}
	duplicateUNP := base
	duplicateUNP.ID = "sale-3"
	duplicateUNP.ExternalID = "order-3"
	if err := r.PutSale(duplicateUNP); !errors.Is(err, ErrSaleUNPConflict) {
		t.Fatalf("duplicate UNP accepted: %v", err)
	}
	otherTenant := base
	otherTenant.ID = "sale-4"
	otherTenant.TenantID = "tenant-b"
	if err := r.PutSale(otherTenant); err != nil {
		t.Fatalf("tenant-scoped identity rejected: %v", err)
	}
}

func TestConcurrentCreateSaleExternalIDHasSingleWinner(t *testing.T) {
	r := NewMemoryRepository()
	s := NewService(r, nil)
	const workers = 24
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.CreateSale(CreateSale{ExternalID: "same-order", RegisterID: "register-1", OperatorID: "A001"})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrSaleExternalIDConflict):
			conflicts++
		default:
			t.Fatalf("unexpected create result: %v", err)
		}
	}
	if winners != 1 || conflicts != workers-1 || len(r.Sales("")) != 1 {
		t.Fatalf("identity race not serialized: winners=%d conflicts=%d sales=%d", winners, conflicts, len(r.Sales("")))
	}
}

func TestAddLineSkipsUNPAlreadyPresentInRecoveredSales(t *testing.T) {
	r := NewMemoryRepository()
	now := time.Now().UTC()
	if err := r.PutSale(Sale{ID: "legacy", TenantID: "tenant-a", ExternalID: "legacy-order", RegisterID: "register-1", OperatorID: "A001", UNP: "register-1-A001-0000001", State: "OPEN", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	draft := Sale{ID: "draft", TenantID: "tenant-a", ExternalID: "new-order", RegisterID: "register-1", OperatorID: "A001", State: "DRAFT", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, CreatedAt: now, UpdatedAt: now}
	if err := r.PutSale(draft); err != nil {
		t.Fatal(err)
	}
	got, err := r.AddSaleLineExpected("draft", "tenant-a", 1, SaleLine{LineID: "line-1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"})
	if err != nil || got.UNP != "register-1-A001-0000002" {
		t.Fatalf("UNP collision was not skipped: unp=%q err=%v", got.UNP, err)
	}
}
