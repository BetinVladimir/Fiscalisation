package domain

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Cents stores monetary amounts as integer hundredths of a currency unit.
// Using int64 avoids floating-point imprecision and overflow-safe arithmetic.
type Cents int64

// ParseMoney parses a decimal string like "12.34" into Cents.
//
// Allowed formats:
//   - Non-negative integers with exactly 2 decimal places: "12.34"
//   - Leading zeros permitted: "01.00" → 100
//   - Integer part must fit in 7 digits (max 9999999.99)
//   - Fractional part must be exactly 2 digits [00..99]
//
// Rejected:
//   - Signed values (leading +/-)
//   - Wrong number of decimal places (".00", "1.5", "1.500")
//   - Empty strings, leading dots, non-numeric characters
func ParseMoney(v string) (Cents, error) {
	// Reject signed values (BUG-001 invariant)
	if len(v) == 0 {
		return 0, errors.New("invalid money")
	}
	if v[0] == '-' || v[0] == '+' {
		return 0, errors.New("invalid money")
	}
	// Must contain exactly one dot
	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return 0, errors.New("invalid money")
	}
	whole, frac := parts[0], parts[1]
	// Fractional part must be exactly 2 digits
	if len(frac) != 2 {
		return 0, errors.New("invalid money: fractional part must be exactly 2 digits")
	}
	// Integer part must be non-empty and at most 7 digits
	if len(whole) == 0 || len(whole) > 7 {
		return 0, errors.New("invalid money")
	}
	wholeStr := whole // may be "01"
	// Parse both parts
	wholeInt, err := strconv.ParseInt(wholeStr, 10, 64)
	if err != nil {
		return 0, err
	}
	fracInt, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, err
	}
	// Validate fraction range
	if fracInt < 0 || fracInt >= 100 {
		return 0, errors.New("invalid money: fractional part must be 00-99")
	}
	return Cents(wholeInt*100 + fracInt), nil
}

// FormatMoney converts Cents to a decimal string with exactly 2 decimal places.
//
// Negative values are represented with a leading minus sign.
func FormatMoney(c Cents) string {
	if c < 0 {
		return "-" + FormatMoney(-c)
	}
	v := int64(c)
	frac := v % 100
	whole := v / 100
	if frac < 10 {
		return strconv.FormatInt(whole, 10) + ".0" + strconv.FormatInt(frac, 10)
	}
	return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(frac, 10)
}

// LineTotal computes the line total from a unit price (in Cents) and a
// quantity in mill units (qty_milli = quantity × 1000), using half-up
// rounding: (price * qty_milli + 500) / 1000.
//
// It returns an error if:
//   - price < 0 or qty <= 0
//   - any intermediate computation overflows int64
func LineTotal(price Cents, qtyMilli int64) (Cents, error) {
	if price < 0 {
		return 0, errors.New("invalid price")
	}
	if qtyMilli <= 0 {
		return 0, errors.New("invalid quantity")
	}
	p := int64(price)
	// Overflow check for p * qtyMilli
	if p > 0 && qtyMilli > (math.MaxInt64-500)/p {
		return 0, errors.New("line overflow")
	}
	return Cents((p*qtyMilli + 500) / 1000), nil
}

// Extended validation helpers (moved from service.go)
func validTax(v string) bool { return len(v) == 1 && strings.Contains("ABCDEFGH", v) }
func validMoney(m Money) bool {
	if m.Currency != "EUR" {
		return false
	}
	if !moneyPattern.MatchString(m.Amount) {
		return false
	}
	dot := strings.IndexByte(m.Amount, '.')
	return dot >= 1 && dot <= 12
}
func validQuantity(v string) bool { x, e := parseFixed(v, 3); return e == nil && x > 0 }
func parseFixed(v string, scale int) (int64, error) {
	parts := strings.Split(v, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid decimal")
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > scale {
		return 0, errors.New("scale")
	}
	for len(frac) < scale {
		frac += "0"
	}
	return strconv.ParseInt(parts[0]+frac, 10, 64)
}
func lineTotalCents(price Money, quantity string) (int64, error) {
	p, err := parseFixed(price.Amount, 2)
	if err != nil || p < 0 {
		return 0, errors.New("invalid price")
	}
	q, err := parseFixed(quantity, 3)
	if err != nil || q <= 0 || (p > 0 && q > (math.MaxInt64-500)/p) {
		return 0, errors.New("invalid quantity or line overflow")
	}
	return (p*q + 500) / 1000, nil
}
func discountedLineTotalCents(line SaleLine) (int64, error) {
	gross, err := lineTotalCents(line.UnitPrice, line.Quantity)
	if err != nil {
		return 0, err
	}
	if line.Discount == nil {
		return gross, nil
	}
	discount, err := parseFixed(line.Discount.Amount, 2)
	if err != nil || line.Discount.Currency != "EUR" || discount < 0 || discount > gross {
		return 0, errors.New("invalid line discount")
	}
	return gross - discount, nil
}
func validDiscount(v *Money) bool {
	if v == nil {
		return true
	}
	amount, err := parseFixed(v.Amount, 2)
	return err == nil && amount >= 0 && v.Currency == "EUR"
}
func saleTotal(s Sale) (int64, error) {
	var total int64
	for _, l := range s.Lines {
		line, err := discountedLineTotalCents(l)
		if err != nil || line > math.MaxInt64-total {
			return 0, errors.New("sale total overflow")
		}
		total += line
	}
	return total, nil
}
func formatFixed(v int64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
