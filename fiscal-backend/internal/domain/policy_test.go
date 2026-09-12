package domain

import (
	"testing"
	"time"
)

func TestDefaultBGPolicyIsEffectiveDatedAndConservative(t *testing.T) {
	c := DefaultBGPolicyCatalog()
	p, err := c.Policy(time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC))
	if err != nil || p.Country != "BG" || p.OfficialCurrency != "EUR" || p.Version != "bg-2026.08.07" || len(p.SourceSHA256) != 64 {
		t.Fatalf("unexpected policy: %#v err=%v", p, err)
	}
	groups, err := c.TaxGroups(time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC))
	if err != nil || len(groups) != 4 || groups[0].Code != "A" || groups[0].Rate != "0.00" || groups[1].Code != "B" || groups[1].Rate != "20.00" || groups[2].Code != "C" || groups[2].Rate != "20.00" || groups[3].Code != "D" || groups[3].Rate != "9.00" {
		t.Fatalf("unexpected tax groups: %#v err=%v", groups, err)
	}
	if c.AllowsTaxGroup("E", time.Now().UTC()) {
		t.Fatal("an unreviewed tax mapping must never become executable")
	}
	if _, err := c.Policy(time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)); err == nil {
		t.Fatal("policy must not apply before valid_from")
	}
}
