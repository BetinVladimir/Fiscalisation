package domain

import (
	"testing"
)

// stubInsufficientCashDriver returns INSUFFICIENT_CASH on every Execute call.
type stubInsufficientCashDriver struct{}

func (d *stubInsufficientCashDriver) Execute(_ Operation, _ Sale, _ PaymentRequest) (string, string) {
	return "", "INSUFFICIENT_CASH"
}
func (d *stubInsufficientCashDriver) Probe() error { return nil }

// completedSaleForReversal creates a COMPLETED sale using the normal simulator,
// then swaps the driver to the provided one before returning the service.
func completedSaleForReversal(t *testing.T, reason string) (*Service, string) {
	t.Helper()
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetSimulated(true)
	sale, err := s.CreateSale(CreateSale{ExternalID: "nap02-sale", RegisterID: "r1", OperatorID: "A001"})
	if err != nil {
		t.Fatalf("CreateSale: %v", err)
	}
	_, err = s.AddLine(sale.ID, SaleLine{
		LineID: "l1", Name: "Item", Quantity: "1.000",
		UnitPrice: Money{Amount: "12.00", Currency: "EUR"}, TaxGroup: "B",
	})
	if err != nil {
		t.Fatalf("AddLine: %v", err)
	}
	_, err = s.Pay(sale.ID, PaymentRequest{
		PaymentID: "p1", Type: "CASH",
		Amount: Money{Amount: "12.00", Currency: "EUR"},
	})
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}
	// Inject insufficient-cash driver before reversal.
	s.driver = &stubInsufficientCashDriver{}
	return s, sale.ID
}

// TestReversalInsufficientCash_ReturnsTypedError — NAP-02: FD -104010
// (ERR_NEED_ALL_PAIDOUT) must surface as INSUFFICIENT_CASH error code.
func TestReversalInsufficientCash_ReturnsTypedError(t *testing.T) {
	s, saleID := completedSaleForReversal(t, "OPERATOR_ERROR")
	op, err := s.ReverseForTenant(saleID, "OPERATOR_ERROR", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.ErrorCode != "INSUFFICIENT_CASH" {
		t.Errorf("ErrorCode = %q; want INSUFFICIENT_CASH", op.ErrorCode)
	}
	if op.State != "FAILED" {
		t.Errorf("State = %q; want FAILED", op.State)
	}
}

// TestReversalInsufficientCash_SaleStaysCompleted — sale must not be marked
// REVERSED or CANCELLED when the storno is rejected for insufficient cash.
func TestReversalInsufficientCash_SaleStaysCompleted(t *testing.T) {
	s, saleID := completedSaleForReversal(t, "OPERATOR_ERROR")
	s.ReverseForTenant(saleID, "OPERATOR_ERROR", "") //nolint:errcheck
	sale, err := s.GetSale(saleID)
	if err != nil {
		t.Fatalf("GetSale: %v", err)
	}
	if sale.State != "COMPLETED" {
		t.Errorf("sale.State = %q; want COMPLETED (must not be reversed)", sale.State)
	}
}

// TestReversalInsufficientCash_OperatorErrorHasCashInAction — when the reversal
// basis is OPERATOR_ERROR, AllowedActions must include CASH_IN so the cashier
// knows to deposit cash and retry.
func TestReversalInsufficientCash_OperatorErrorHasCashInAction(t *testing.T) {
	s, saleID := completedSaleForReversal(t, "OPERATOR_ERROR")
	op, err := s.ReverseForTenant(saleID, "OPERATOR_ERROR", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, a := range op.AllowedActions {
		if a == "CASH_IN" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AllowedActions = %v; want to contain CASH_IN (deposit cash and retry)", op.AllowedActions)
	}
}
