package domain

import "errors"

// ErrZMismatch is returned by ValidateZTotals when the FU-reported totals differ
// from the backend-computed totals. This triggers a Z_MISMATCH incident.
var ErrZMismatch = errors.New("Z_MISMATCH")

// ZReportTotals holds the totals from a Z (close-of-day) fiscal report.
// All monetary fields are in whole cents (invariant 1).
type ZReportTotals struct {
	NReport   int64 // report sequence number from the FU response
	TotA      int64 // total taxable amount A (cents)
	TotB      int64 // total taxable amount B (cents)
	TotC      int64 // total taxable amount C (cents)
	TotD      int64 // total taxable amount D (cents)
	CashTotal int64 // total cash collected (cents)
	CardTotal int64 // total card collected (cents)
}

// ValidateZTotals compares reported (FU) totals with backend-computed totals.
// Returns ErrZMismatch if any field differs.
func ValidateZTotals(reported, backend ZReportTotals) error {
	if reported.TotA != backend.TotA ||
		reported.TotB != backend.TotB ||
		reported.TotC != backend.TotC ||
		reported.TotD != backend.TotD ||
		reported.CashTotal != backend.CashTotal ||
		reported.CardTotal != backend.CardTotal {
		return ErrZMismatch
	}
	return nil
}
