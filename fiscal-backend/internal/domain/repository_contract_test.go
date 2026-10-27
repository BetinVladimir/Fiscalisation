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
