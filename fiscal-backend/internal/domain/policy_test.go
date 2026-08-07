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
	if err != nil || len(groups) != 1 || groups[0].Code != "B" || groups[0].Rate != "20.00" {
		t.Fatalf("unexpected tax groups: %#v err=%v", groups, err)
	}
	if c.AllowsTaxGroup("A", time.Now().UTC()) {
		t.Fatal("an unreviewed tax mapping must never become executable")
	}
	if _, err := c.Policy(time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)); err == nil {
		t.Fatal("policy must not apply before valid_from")
	}
}
