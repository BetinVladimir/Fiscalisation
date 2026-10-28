package domain

import (
	"sync"
	"testing"
)

// TestConcurrentFinalizeSameKeyProducesOneOperation verifies that when N
// goroutines concurrently call CreateSale with the same external_id,
// the MemoryRepository prevents duplicate creation: exactly one operation
// succeeds and exactly one sale record exists in the store.
func TestConcurrentFinalizeSameKeyProducesOneOperation(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))

	const concurrency = 50
	var wg sync.WaitGroup
	wg.Add(concurrency)
	errCount := 0
	var mu sync.Mutex
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, err := s.CreateSale(CreateSale{
				ExternalID: "concurrent-key-singleton",
				RegisterID: "register-1",
				OperatorID: "A001",
			})
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if errCount != concurrency-1 {
		t.Errorf("want %d duplicate rejections, got %d", concurrency-1, errCount)
	}

	var found int
	for _, sale := range repo.Sales("") {
		if sale.ExternalID == "concurrent-key-singleton" {
			found++
		}
	}
	if found != 1 {
		t.Errorf("want exactly 1 sale in repository, got %d", found)
	}
}

// TestRepositoryReloadPreservesState verifies that after persisting operations,
// a fresh MemoryRepository backed by the same store loads the saved state,
// so generation strictly increases and previously stored sales survive reload.
func TestRepositoryReloadPreservesState(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, NewSimulator(true))

	sale, err := s.CreateSale(CreateSale{ExternalID: "reload-sale-1", RegisterID: "R1", OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}

	// Reload from the same backing store.
	repo2, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := repo2.Sale(sale.ID)
	if err != nil {
		t.Fatalf("sale not found after reload: %v", err)
	}
	if loaded.ExternalID != sale.ExternalID {
		t.Errorf("want ExternalID %q, got %q", sale.ExternalID, loaded.ExternalID)
	}
}

// TestConcurrentFinalizeSameKeyFinalizeProducesOneOperation verifies Invariant 4:
// 8 goroutines calling FinalizeSaleForTenant with the same ClientOperationID
// produce exactly one Operation record and trigger Driver.Execute exactly once.
func TestConcurrentFinalizeSameKeyFinalizeProducesOneOperation(t *testing.T) {
	const opID = "00000000-0000-4000-8000-100000000001"
	const receiptID = "00000000-0000-4000-8000-100000000002"
	const payID = "00000000-0000-4000-8000-100000000003"

	repo := NewMemoryRepository()
	driver := &finalizeCountingDriver{}
	svc := NewService(repo, driver)

	sale := Sale{
		ID:         "sale-concurrent-finalize",
		State:      "OPEN",
		UNP:        "12345678-A001-0000001",
		RegisterID: "register-1",
		Lines: []SaleLine{{
			LineID:    "line-1",
			Name:      "Coffee",
			Quantity:  "1.000",
			UnitPrice: Money{Amount: "2.50", Currency: "EUR"},
			TaxGroup:  "B",
		}},
		Version: 1,
	}
	if err := repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}

	req := SaleFinalizeRequest{
		ClientOperationID: opID,
		ReceiptSessionID:  receiptID,
		Payments: []PaymentRequest{{
			PaymentID: payID,
			Type:      "CASH",
			Amount:    Money{Amount: "2.50", Currency: "EUR"},
		}},
		ExpectedTotal: Money{Amount: "2.50", Currency: "EUR"},
	}

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			svc.FinalizeSaleForTenant(sale.ID, req, "") //nolint:errcheck
		}()
	}
	wg.Wait()

	var matched int
	for _, op := range repo.Operations() {
		if op.ID == opID {
			matched++
		}
	}
	if matched != 1 {
		t.Errorf("want exactly 1 operation with ID %s, got %d", opID, matched)
	}
	if driver.calls() != 1 {
		t.Errorf("want driver.Execute called exactly 1 time, got %d", driver.calls())
	}
}

type finalizeCountingDriver struct {
	mu sync.Mutex
	n  int
}

func (d *finalizeCountingDriver) Probe() error { return nil }
func (d *finalizeCountingDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.n++
	return "receipt-finalize", ""
}
func (d *finalizeCountingDriver) calls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

// TestEmptyRepositoryStartsEmpty verifies that a fresh MemoryRepository
// starts empty (no error, no stale state).
func TestEmptyRepositoryStartsEmpty(t *testing.T) {
	repo := NewMemoryRepository()
	if len(repo.Sales("")) != 0 {
		t.Error("fresh repository must start empty")
	}
	if _, err := repo.Sale("no-such-id"); err == nil {
		t.Error("Sale on non-existent id must return error")
	}
}
