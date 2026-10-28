package domain

import "testing"

// PaymentTypeCapabilityMatrix verifies the capability contract for all 8 SUPTO payment types.
func TestPaymentTypeCapabilityMatrix(t *testing.T) {
	type row struct {
		paymentType  string
		capabilities []string
		wantErr      bool
	}
	tests := []row{
		// Base types — always allowed regardless of capabilities
		{"CASH", nil, false},
		{"CARD", nil, false},
		{"CASH", []string{"NHIF_PAYMENT"}, false},
		{"CARD", []string{}, false},

		// Extended types — rejected without capability
		{"NHIF", nil, true},
		{"NHIF", []string{"CHEQUE_PAYMENT"}, true},
		{"NHIF", []string{"NHIF_PAYMENT"}, false},

		{"CHEQUE", nil, true},
		{"CHEQUE", []string{"CHEQUE_PAYMENT"}, false},

		{"VOUCHER", nil, true},
		{"VOUCHER", []string{"VOUCHER_PAYMENT"}, false},

		{"DEFERRED", nil, true},
		{"DEFERRED", []string{"DEFERRED_PAYMENT"}, false},

		{"INTERNAL_CONSUMPTION", nil, true},
		{"INTERNAL_CONSUMPTION", []string{"INTERNAL_CONSUMPTION_PAYMENT"}, false},

		{"COUPON", nil, true},
		{"COUPON", []string{"COUPON_PAYMENT"}, false},
	}
	for _, tc := range tests {
		err := ValidatePaymentTypeForDevice(tc.paymentType, tc.capabilities)
		got := err != nil
		if got != tc.wantErr {
			t.Errorf("ValidatePaymentTypeForDevice(%q, %v) error=%v; want error=%v (err=%v)",
				tc.paymentType, tc.capabilities, got, tc.wantErr, err)
		}
		if err != nil && err.Error() != "PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE" {
			t.Errorf("wrong error message: %v", err)
		}
	}
}
