package domain

import (
	"testing"
)

// TestReceiptMoneyCentsRejectsSignedAndMalformed verifies receiptMoneyCents
// rejects signed/malformed inputs and accepts valid ones.
func TestReceiptMoneyCentsRejectsSignedAndMalformed(t *testing.T) {
	errTests := []string{"-0.50", "-1.50", "+1.00", "1.-5", "1.5", "", "1e2.00", " 1.00", "1.000"}
	for _, in := range errTests {
		_, err := receiptMoneyCents(in)
		if err == nil {
			t.Errorf("receiptMoneyCents(%q) expected error, got nil", in)
		}
	}
	okTests := []struct {
		in   string
		want int64
	}{
		{"0.00", 0},
		{"0.01", 1},
		{"12.34", 1234},
		{"99999.99", 9999999},
		{"1.50", 150},
		{"01.00", 100},
	}
	for _, tc := range okTests {
		got, err := receiptMoneyCents(tc.in)
		if err != nil {
			t.Errorf("receiptMoneyCents(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("receiptMoneyCents(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestValidateReceiptPlanRejectsNegative verifies ValidateReceiptPlan rejects
// negative payment amounts even when they sum correctly to expected.
func TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly(t *testing.T) {
	plan := ReceiptFinalizePlan{
		ClientOperationID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		ReceiptSessionID:  "b2c3d4e5-f6a7-8901-bcde-f12345678901",
		SaleID:            "sale-001",
		UNP:               "UNP12345",
		Items:             []SaleLine{{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "10.00", Currency: "EUR"}, TaxGroup: "TAX"}},
		Payments: []PaymentRequest{
			{PaymentID: "pay-001", Type: "CARD", Amount: Money{Amount: "-0.50", Currency: "EUR"}},
			{PaymentID: "pay-002", Type: "CASH", Amount: Money{Amount: "11.00", Currency: "EUR"}},
		},
		ExpectedTotal: Money{Amount: "10.50", Currency: "EUR"},
	}
	err := ValidateReceiptPlan(plan)
	if err == nil {
		t.Errorf("ValidateReceiptPlan expected error for negative payment, got nil")
	}
}

// TestValidateReceiptPlanRejectsItemsTotalMismatch verifies ValidateReceiptPlan
// rejects when the sum of discounted line totals does not match expected total.
func TestValidateReceiptPlanRejectsItemsTotalMismatch(t *testing.T) {
	plan := ReceiptFinalizePlan{
		ClientOperationID: "c3d4e5f6-a7b8-9012-cdef-345678901234",
		ReceiptSessionID:  "d4e5f6a7-b8c9-0123-defa-456789012345",
		SaleID:            "sale-m",
		UNP:               "UNP67890",
		Items: []SaleLine{
			{LineID: "l1", Name: "Widget", Quantity: "1.000", UnitPrice: Money{Amount: "10.00", Currency: "EUR"}, TaxGroup: "TAX"},
		},
		Payments: []PaymentRequest{
			{PaymentID: "pay-001", Type: "CASH", Amount: Money{Amount: "5.00", Currency: "EUR"}},
			{PaymentID: "pay-002", Type: "CARD", Amount: Money{Amount: "5.50", Currency: "EUR"}},
		},
		ExpectedTotal: Money{Amount: "10.50", Currency: "EUR"},
	}
	err := ValidateReceiptPlan(plan)
	if err == nil {
		t.Errorf("ValidateReceiptPlan expected items total mismatch error, got nil")
	}
}
