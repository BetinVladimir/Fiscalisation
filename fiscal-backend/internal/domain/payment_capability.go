package domain

import "errors"

// paymentTypeCapabilities maps payment types that require explicit device
// capability to the capability string that must be present in the binding.
// CASH and CARD are universally supported; all others need a declared capability.
var paymentTypeCapabilities = map[string]string{
	"CHEQUE":               "CHEQUE_PAYMENT",
	"VOUCHER":              "VOUCHER_PAYMENT",
	"DEFERRED":             "DEFERRED_PAYMENT",
	"NHIF":                 "NHIF_PAYMENT",
	"INTERNAL_CONSUMPTION": "INTERNAL_CONSUMPTION_PAYMENT",
	"COUPON":               "COUPON_PAYMENT",
}

// ValidatePaymentTypeForDevice returns PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE if
// the payment type requires a capability the device does not declare.
// Must be called before any I/O to the fiscal unit.
func ValidatePaymentTypeForDevice(paymentType string, deviceCapabilities []string) error {
	required, needsCap := paymentTypeCapabilities[paymentType]
	if !needsCap {
		return nil // CASH, CARD — always allowed
	}
	for _, c := range deviceCapabilities {
		if c == required {
			return nil
		}
	}
	return errors.New("PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE")
}
