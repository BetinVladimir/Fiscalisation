package domain

import (
	"strings"
	"testing"
)

// TestSpecialUNPFormats verifies that NewSpecialUNP produces the canonical
// formats mandated by Наредба Н-18 Прил. 29 for reversals without an original
// SUPTO document.
//
// MARKET_WITHDRAWAL → "XX000000-0000-0000000"  (чл. 31(11))
// PRE_SUPTO_SALE    → "OO000000-0000-0000000"  (чл. 31(12) / Прил. 29 п.10)
//
// All letters are Latin (not Cyrillic). The digit groups are zeros.
func TestSpecialUNPFormats(t *testing.T) {
	cases := []struct {
		kind  SpecialUNPKind
		want  string
		descr string
	}{
		{SpecialUNPMarketWithdrawal, "XX000000-0000-0000000", "market withdrawal (чл. 31(11))"},
		{SpecialUNPPreSUPTOSale, "OO000000-0000-0000000", "pre-SUPTO sale (чл. 31(12))"},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			u, err := NewSpecialUNP(tc.kind, "AB")
			if err != nil {
				t.Fatalf("NewSpecialUNP(%q, %q): %v", tc.kind, "AB", err)
			}
			if u.Kind != tc.kind {
				t.Errorf("Kind = %q; want %q", u.Kind, tc.kind)
			}
			got := u.Value()
			if got != tc.want {
				t.Errorf("Value() = %q; want %q (%s)", got, tc.want, tc.descr)
			}
			// Confirm letters are Latin: no rune > 127 in the prefix
			for _, r := range u.Prefix {
				if r > 127 {
					t.Errorf("non-ASCII rune %U in prefix %q — must be Latin", r, u.Prefix)
				}
			}
			// Confirm structure: 8-4-7
			parts := strings.Split(got, "-")
			if len(parts) != 3 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 7 {
				t.Errorf("malformed UNP structure %q (want 8-4-7)", got)
			}
		})
	}
	// Unknown kind must return an error
	if _, err := NewSpecialUNP("UNKNOWN_KIND", "AB"); err == nil {
		t.Error("expected error for unknown SpecialUNP kind, got nil")
	}
}

// TestReversalWithoutOriginalRequiresBasis verifies that only recognised legal
// bases (MARKET_WITHDRAWAL, PRE_SUPTO_SALE) are admitted as basis reasons for
// special-UNP reversals, and that ordinary reason codes are rejected.
func TestReversalWithoutOriginalRequiresBasis(t *testing.T) {
	allowed := []string{
		string(SpecialUNPMarketWithdrawal),
		string(SpecialUNPPreSUPTOSale),
	}
	for _, r := range allowed {
		if !ReversalBasisAllowed(r) {
			t.Errorf("ReversalBasisAllowed(%q) = false; want true", r)
		}
	}

	disallowed := []string{
		"OPERATOR_ERROR",
		"CLIENT_REQUEST",
		"TECHNICAL_ISSUE",
		"",
		"market_withdrawal", // lowercase must not match
		"PRE-SUPTO-SALE",    // wrong separator
	}
	for _, r := range disallowed {
		if ReversalBasisAllowed(r) {
			t.Errorf("ReversalBasisAllowed(%q) = true; want false", r)
		}
	}
}
