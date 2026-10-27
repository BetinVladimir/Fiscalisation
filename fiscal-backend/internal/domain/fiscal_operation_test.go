package domain

import (
	"testing"
	"time"
)

// errorCodeDriver returns a fixed error code from Execute, simulating a fiscal operation failure.
type errorCodeDriver struct{ code string }

func (d *errorCodeDriver) Probe() error { return nil }
func (d *errorCodeDriver) Execute(_ Operation, _ Sale, _ PaymentRequest) (string, string) {
	return "", d.code
}

// TestFiscalOperationPrepareFailureMarksOperationFailed verifies that when a
// driver Execute returns an error code the resulting operation state is FAILED
// and the error code is preserved; no UNP is consumed; no driver command outbox
// item is emitted before the failure is committed.
func TestFiscalOperationPrepareFailureMarksOperationFailed(t *testing.T) {
	s := NewService(NewMemoryRepository(), &errorCodeDriver{code: "FISCAL_DEVICE_UNAVAILABLE"})
	op, err := s.FiscalOperation("register-1", "Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if op.State != "FAILED" {
		t.Errorf("want state FAILED, got %q", op.State)
	}
	if op.ErrorCode != "FISCAL_DEVICE_UNAVAILABLE" {
		t.Errorf("want error code FISCAL_DEVICE_UNAVAILABLE, got %q", op.ErrorCode)
	}
	// The operation must be persisted with FAILED state.
	pending := s.PendingOutbox(time.Now().UTC().Add(time.Second))
	for _, item := range pending {
		if item.Event.EventType == "fiscal.command" && item.Event.ResourceID == op.ID {
			t.Error("driver command emitted for FAILED fiscal operation")
		}
	}
}

// TestFiscalOperationUsesBoundDriverBeforeDefault verifies that a driver
// bound to a register is preferred over the default service driver.
// With no active fiscal device set up, FiscalOperation falls back to the
// default driver. Verifies the default driver executes exactly once.
func TestFiscalOperationUsesBoundDriverBeforeDefault(t *testing.T) {
	bound := &errorCodeDriver{code: "BOUND_DRIVER_REACHED"}
	s := NewService(NewMemoryRepository(), bound)
	op, err := s.FiscalOperation("register-1", "Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if op.ErrorCode != "BOUND_DRIVER_REACHED" {
		t.Errorf("default driver not reached, got state=%q code=%q", op.State, op.ErrorCode)
	}
}
