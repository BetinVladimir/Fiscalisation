// This file is a lint rule testdata violator — it intentionally uses float64 in a money context.
// It is NOT compiled into the main build. Used by scripts/check-lint-rules.sh to verify
// that the money-float rule fires correctly (exit ≠ 0 when this file is in scope).
// STYLE-001 sentinel: float64_in_domain_money_path
package lint

// violatingPriceCalc uses float64 for price — this is the FORBIDDEN pattern.
func violatingPriceCalc(unitCents float64, qty int) float64 {
	return unitCents * float64(qty)
}
