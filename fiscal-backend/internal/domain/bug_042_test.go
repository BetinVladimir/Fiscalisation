package domain

import "testing"

// TestNHIFPaymentWithoutCapabilityRejectedBeforeIO verifies that NHIF payment
// on a device without the NHIF_PAYMENT capability is rejected with
// PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE before any I/O occurs.
func TestNHIFPaymentWithoutCapabilityRejectedBeforeIO(t *testing.T) {
	err := ValidatePaymentTypeForDevice("NHIF", nil)
	if err == nil {
		t.Fatal("expected PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE, got nil")
	}
	if err.Error() != "PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE" {
		t.Fatalf("expected PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE, got %q", err.Error())
	}
}

// TestNHIFPaymentWithCapabilityAllowed verifies NHIF is allowed when device
// declares NHIF_PAYMENT capability.
func TestNHIFPaymentWithCapabilityAllowed(t *testing.T) {
	if err := ValidatePaymentTypeForDevice("NHIF", []string{"NHIF_PAYMENT"}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
