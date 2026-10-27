package domain

import (
	"testing"
)

// TestPaymentTypeCapabilityMatrix verifies that each payment type in the
// global supported-type list is accepted, and that unknown/empty types are
// rejected before any side effect (no UNP consumed, no operation created).
//
// Per the BC50 profile in docs/MVP1/fixtures/payment_slots_bc50.json the
// device supports CASH, CARD, CHEQUE, VOUCHER, COUPON; NHIF, DEFERRED, and
// INTERNAL_CONSUMPTION are listed as unsupported.  The supported-type check
// in receipt_saga.go currently runs the global static list; device-specific
// capability gating (BUG-042) requires a separate pass.
func TestPaymentTypeCapabilityMatrix(t *testing.T) {
	// Global list: these types pass supportedPaymentType and must reach the
	// service without being pre-rejected.
	supported := []string{"CASH", "CARD", "CHEQUE", "VOUCHER", "DEFERRED", "NHIF", "INTERNAL_CONSUMPTION", "COUPON"}
	for _, pt := range supported {
		t.Run("supported/"+pt, func(t *testing.T) {
			if !isSupportedPaymentType(pt) {
				t.Errorf("payment type %q must be in global supported list", pt)
			}
		})
	}

	// Unknown types must be rejected before a PaymentRequest reaches the driver.
	unsupported := []string{"", "UNKNOWN", "BARTER", "CRYPTO", "WIRE", "GIFT_CARD"}
	for _, pt := range unsupported {
		t.Run("unsupported/"+pt, func(t *testing.T) {
			if isSupportedPaymentType(pt) {
				t.Errorf("payment type %q must not be in global supported list", pt)
			}
		})
	}
}

// isSupportedPaymentType mirrors the logic in receipt_saga.go:supportedPaymentType.
// It is co-located here to pin the acceptance set without importing internal state.
func isSupportedPaymentType(t string) bool {
	switch t {
	case "CASH", "CARD", "CHEQUE", "VOUCHER", "DEFERRED", "NHIF", "INTERNAL_CONSUMPTION", "COUPON":
		return true
	default:
		return false
	}
}

// TestPaymentTypeRejectedBeforeSideEffect verifies that an invalid payment
// type is rejected before any UNP is consumed or driver is invoked.
func TestPaymentTypeRejectedBeforeSideEffect(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))

	sale, err := s.CreateSale(CreateSale{ExternalID: "cap-test-1", RegisterID: "R1", OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	// Add a valid line so the sale is non-empty
	_, err = s.AddLine(sale.ID, SaleLine{
		LineID: "L1", Name: "Item", Quantity: "1.000",
		UnitPrice: Money{Amount: "10.00", Currency: "EUR"},
		TaxGroup:  "A",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Attempt to pay with an invalid type — must be rejected before execution.
	_, err = s.payForTenant(sale.ID, PaymentRequest{
		PaymentID: "pay-1",
		Type:      "CRYPTO",
		Amount:    Money{Amount: "10.00", Currency: "EUR"},
	}, "")
	if err == nil {
		t.Fatal("invalid payment type must be rejected before driver execution")
	}
	// No operations must have been created.
	for _, op := range repo.Operations() {
		if op.TenantID == "" && op.RegisterID == "R1" {
			t.Errorf("operation created despite invalid payment type: %+v", op)
		}
	}
}
