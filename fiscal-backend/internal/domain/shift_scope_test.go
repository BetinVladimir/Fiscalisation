package domain

import (
	"sync"
	"testing"
)

func TestShiftCloseIgnoresUnresolvedOperationsFromOtherTenantOrRegister(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   Operation
	}{
		{"other tenant", Operation{ID: "op-other-tenant", TenantID: "tenant-b", RegisterID: "register-a", State: "UNKNOWN", Version: 1}},
		{"other register", Operation{ID: "op-other-register", TenantID: "tenant-a", RegisterID: "register-b", State: "RECONCILING", Version: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewMemoryRepository()
			shift, err := repo.OpenShift("register-a", "operator-a", "tenant-a")
			if err != nil {
				t.Fatal(err)
			}
			if err = repo.PutOperation(tc.op); err != nil {
				t.Fatal(err)
			}
			closed, err := repo.CloseShift(shift.ID)
			if err != nil || closed.State != "CLOSED" {
				t.Fatalf("unrelated unresolved operation blocked shift: shift=%+v operation=%+v err=%v", closed, tc.op, err)
			}
		})
	}
}

func TestShiftCloseBlocksEveryUnresolvedStateOnSameTenantRegister(t *testing.T) {
	for _, state := range []string{"EXECUTING", "UNKNOWN", "RECONCILING"} {
		t.Run(state, func(t *testing.T) {
			repo := NewMemoryRepository()
			shift, err := repo.OpenShift("register-a", "operator-a", "tenant-a")
			if err != nil {
				t.Fatal(err)
			}
			op := Operation{ID: "op-" + state, TenantID: "tenant-a", RegisterID: "register-a", State: state, Version: 1}
			if err = repo.PutOperation(op); err != nil {
				t.Fatal(err)
			}
			blocked, err := repo.CloseShift(shift.ID)
			if err == nil || blocked.State != "RECONCILIATION_REQUIRED" {
				t.Fatalf("same-register %s operation did not block close: %+v %v", state, blocked, err)
			}
		})
	}
}

func TestLegacySaleOperationDerivesRegisterForShiftClose(t *testing.T) {
	repo := NewMemoryRepository()
	shift, _ := repo.OpenShift("register-a", "operator-a", "tenant-a")
	sale := Sale{ID: "sale-a", TenantID: "tenant-a", RegisterID: "register-a", State: "UNKNOWN", Version: 1}
	if err := repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	if err := repo.PutOperation(Operation{ID: "legacy-op", TenantID: "tenant-a", SaleID: sale.ID, State: "UNKNOWN", Version: 1}); err != nil {
		t.Fatal(err)
	}
	blocked, err := repo.CloseShift(shift.ID)
	if err == nil || blocked.State != "RECONCILIATION_REQUIRED" {
		t.Fatalf("legacy sale operation register was not derived: %+v %v", blocked, err)
	}
}

func TestConcurrentOpenShiftAllowsExactlyOnePerTenantRegister(t *testing.T) {
	repo := NewMemoryRepository()
	const attempts = 16
	var wg sync.WaitGroup
	results := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.OpenShift("register-a", "operator-a", "tenant-a")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 || len(repo.Shifts("tenant-a")) != 1 {
		t.Fatalf("concurrent open created %d successful shifts and %d records", successes, len(repo.Shifts("tenant-a")))
	}
}

func TestOpenShiftScopesUniquenessAndBlocksUnresolvedPredecessor(t *testing.T) {
	repo := NewMemoryRepository()
	first, err := repo.OpenShift("register-a", "operator-a", "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.OpenShift("register-a", "operator-b", "tenant-b"); err != nil {
		t.Fatal("same register identifier in another autonomous tenant was blocked", err)
	}
	repo.shifts[first.ID] = Shift{ID: first.ID, TenantID: first.TenantID, RegisterID: first.RegisterID, OperatorID: first.OperatorID, State: "RECONCILIATION_REQUIRED", Version: first.Version + 1}
	if _, err = repo.OpenShift("register-a", "operator-a2", "tenant-a"); err == nil {
		t.Fatal("replacement shift opened over unresolved predecessor")
	}
}
