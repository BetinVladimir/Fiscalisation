package domain

import (
	"errors"
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
