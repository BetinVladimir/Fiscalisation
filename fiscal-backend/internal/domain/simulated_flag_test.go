package domain

import (
	"testing"
	"time"
)

// fakeDriver implements Driver but is not a *Simulator.
type fakeDriver struct{}

func (d *fakeDriver) Execute(op Operation, sale Sale, p PaymentRequest) (string, string) {
	return "00000001", ""
}

func (d *fakeDriver) Probe() error {
	return nil
}

func newFakeDriver() *fakeDriver { return &fakeDriver{} }

// TestOperationSimulatedFlagFollowsDriver verifies that the simulated flag
// in reserved operations comes from the driver type, not a hardcoded value.
// BUG-003: operations on real hardware must have Simulated=false.
func TestOperationSimulatedFlagFollowsDriver(t *testing.T) {
	// Group 1: Simulator driver — operations should be Simulated=true (via SetSimulated).
	t.Run("Simulator driver yields Simulated=true when SetSimulated", func(t *testing.T) {
		repo := NewMemoryRepository()
		sim := NewSimulator(true)
		s := NewService(repo, sim)
		s.SetSimulated(true)

		// Verify the set driver is a *Simulator
		if _, ok := s.driver.(*Simulator); !ok {
			t.Fatal("expected driver to be *Simulator")
		}

		// Create a sale and cancel it to exercise cancelSaleForTenant path
		sale, err := s.CreateSale(CreateSale{ExternalID: "sim-op-1", RegisterID: "FD000001", OperatorID: "A001"})
		if err != nil {
			t.Fatal(err)
		}
		op, err := s.CancelSale(sale.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !op.Simulated {
			t.Errorf("CancelSale with SetSimulated(true): Simulated=%v, want true", op.Simulated)
		}
	})

	// Group 2: non-Simulator driver — operations MUST have Simulated=false regardless of SetSimulated.
	// This is the BUG-003 critical invariant: real hardware = never simulated.
	t.Run("non-Simulator driver yields Simulated=false even with SetSimulated", func(t *testing.T) {
		repo := NewMemoryRepository()
		s := NewService(repo, newFakeDriver())
		s.SetSimulated(true) // Even if someone calls this with a fake driver, it must be ignored.

		sale, err := s.CreateSale(CreateSale{ExternalID: "real-op-1", RegisterID: "FD000001", OperatorID: "A001"})
		if err != nil {
			t.Fatal(err)
		}
		op, err := s.CancelSale(sale.ID)
		if err != nil {
			t.Fatal(err)
		}
		if op.Simulated {
			t.Errorf("CancelSale with fake driver + SetSimulated(true): Simulated=%v, want false", op.Simulated)
		}
	})
}

// TestExportOperationIsNeverSimulated verifies that export operations are always
// Simulated=false regardless of driver or SetSimulated state.
func TestExportOperationIsNeverSimulated(t *testing.T) {
	// With a simulator (and SetSimulated(true)) — exports must still be false.
	t.Run("CreateExport with SetSimulated(true) on simulator", func(t *testing.T) {
		repo := NewMemoryRepository()
		_ = NewSimulator(true)
		s := NewService(repo, newFakeDriver())
		s.SetSimulated(true) // Even with SetSimulated(true) on a fake driver, exports must be false.

		op, err := s.CreateExport(ExportRequest{
			Type:   "SUPTO_18_1",
			Format: "JSON",
			From:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			To:     time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		}, "export-tenant")
		if err != nil {
			t.Fatalf("CreateExport: %v", err)
		}
		if op.Simulated {
			t.Errorf("CreateExport: Simulated=%v, want false", op.Simulated)
		}
	})

	t.Run("CreatePeriodizedExport with SetSimulated(true) on simulator", func(t *testing.T) {
		repo := NewMemoryRepository()
		sim := NewSimulator(true)
		s := NewService(repo, sim)
		s.SetSimulated(true)
		_, _ = prepareBLERegister(t, s, "export-tenant-2")

		op, err := s.CreatePeriodizedExport(ExportRequest{
			Type:   "SUPTO_18_1",
			Format: "JSON",
			From:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			To:     time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		}, "export-tenant-2")
		if err != nil {
			t.Fatalf("CreatePeriodizedExport: %v", err)
		}
		if op.Simulated {
			t.Errorf("CreatePeriodizedExport: Simulated=%v, want false", op.Simulated)
		}
	})
}
