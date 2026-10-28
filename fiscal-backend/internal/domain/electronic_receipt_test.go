package domain

import "testing"

// TestEReceiptNegativeGate verifies that SendEReceipt permanently rejects every
// request with ERECEIPT_UNSUPPORTED. This is the technical gate ensuring that
// e-receipt delivery is never accidentally introduced in the MVP1 codebase.
func TestEReceiptNegativeGate(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))

	err := s.SendEReceipt(EReceiptRequest{SaleID: "sale-1", TenantID: "tenant-a", Email: "test@example.com"})
	if err == nil {
		t.Fatal("SendEReceipt must always return an error — e-receipt is excluded from MVP1")
	}
	erErr, ok := err.(*EReceiptError)
	if !ok {
		t.Fatalf("SendEReceipt must return *EReceiptError, got %T: %v", err, err)
	}
	if erErr.Code != "ERECEIPT_UNSUPPORTED" {
		t.Fatalf("expected error code ERECEIPT_UNSUPPORTED, got %q", erErr.Code)
	}
}

// TestEReceiptNegativeGateEmptyRequest verifies rejection even for empty input.
func TestEReceiptNegativeGateEmptyRequest(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	err := s.SendEReceipt(EReceiptRequest{})
	if err == nil {
		t.Fatal("SendEReceipt must always error regardless of input")
	}
	if e, ok := err.(*EReceiptError); !ok || e.Code != "ERECEIPT_UNSUPPORTED" {
		t.Fatalf("expected *EReceiptError{ERECEIPT_UNSUPPORTED}, got %T: %v", err, err)
	}
}
