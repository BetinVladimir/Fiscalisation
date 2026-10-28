package domain

import (
	"errors"
	"strconv"
	"strings"
)

// maxLineCents is the Datecs/Daisy per-line total limit: 9,999,999.99 EUR = 999999999¢.
const maxLineCents = int64(999_999_999)

var ErrLineLimitExceeded = errors.New("line total exceeds fiscal device limit (9999999.99)")
var ErrNegativeUnitPrice = errors.New("unit price must not be negative")

// ValidateLine returns an error if unit_price × quantity overflows the Datecs/Daisy
// line ceiling or if the unit price is negative. Both values must already satisfy
// their respective format invariants; malformed inputs are silently skipped here
// because validMoney/validQuantity guards them upstream.
func ValidateLine(price Money, qty string) error {
	p := strings.Split(price.Amount, ".")
	if len(p) != 2 {
		return nil
	}
	unitCents, err := strconv.ParseInt(p[0]+p[1], 10, 64)
	if err != nil {
		return nil
	}
	if unitCents < 0 {
		return ErrNegativeUnitPrice
	}
	q := strings.Split(qty, ".")
	if len(q) != 2 {
		return nil
	}
	qtyMilli, err := strconv.ParseInt(q[0]+q[1], 10, 64)
	if err != nil {
		return nil
	}
	lineCents := (unitCents*qtyMilli + 500) / 1000
	if lineCents > maxLineCents {
		return ErrLineLimitExceeded
	}
	return nil
}
