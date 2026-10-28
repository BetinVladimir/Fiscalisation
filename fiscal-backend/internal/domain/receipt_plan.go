package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type ReceiptFinalizePlan struct {
	ClientOperationID string           `json:"client_operation_id"`
	ReceiptSessionID  string           `json:"receipt_session_id"`
	SaleID            string           `json:"sale_id"`
	UNP               string           `json:"unp"`
	Items             []SaleLine       `json:"items"`
	Payments          []PaymentRequest `json:"payments"`
	ExpectedTotal     Money            `json:"expected_total"`
}

var moneyPattern = regexp.MustCompile(`^[0-9]+\.[0-9]{2}$`)

func ReceiptPlanHash(v ReceiptFinalizePlan) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func ValidateReceiptPlan(v ReceiptFinalizePlan) error {
	if !uuidPattern.MatchString(v.ClientOperationID) || !uuidPattern.MatchString(v.ReceiptSessionID) || v.SaleID == "" || v.UNP == "" || len(v.Items) == 0 || len(v.Payments) == 0 || v.ExpectedTotal.Currency != "EUR" {
		return errors.New("invalid receipt finalize plan")
	}
	var total int64
	for _, p := range v.Payments {
		if !uuidPattern.MatchString(p.PaymentID) || !supportedPaymentType(p.Type) || p.Amount.Currency != "EUR" {
			return errors.New("invalid payment plan")
		}
		cents, e := receiptMoneyCents(p.Amount.Amount)
		if e != nil || cents <= 0 {
			return errors.New("invalid payment amount")
		}
		total += cents
	}
	expected, e := receiptMoneyCents(v.ExpectedTotal.Amount)
	if e != nil || expected != total {
		return errors.New("payment total mismatch")
	}
	var itemsTotal int64
	for _, line := range v.Items {
		lt, e := discountedLineTotalCents(line)
		if e != nil {
			return e
		}
		itemsTotal += lt
	}
	if itemsTotal != expected {
		return errors.New("items total mismatch")
	}
	return nil
}

func supportedPaymentType(paymentType string) bool {
	switch paymentType {
	case "CASH", "CARD", "CHEQUE", "VOUCHER", "DEFERRED", "NHIF", "INTERNAL_CONSUMPTION", "COUPON":
		return true
	default:
		return false
	}
}

func receiptMoneyCents(v string) (int64, error) {
	if !moneyPattern.MatchString(v) {
		return 0, errors.New("invalid money")
	}
	s := strings.Replace(v, ".", "", 1)
	whole, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		return 0, e
	}
	return whole, nil
}
