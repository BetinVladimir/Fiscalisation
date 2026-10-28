package emailauth

import (
	"context"
	"testing"
)

func TestOnboardRequiresCurrentBusinessTerms(t *testing.T) {
	service := &Service{}
	if _, err := service.Onboard(context.Background(), "", Onboarding{FullName: "Test User"}); err == nil || err.Error() != "current business terms must be accepted" {
		t.Fatalf("expected legal acceptance error, got %v", err)
	}
}

func TestSplitNameAllowsSingleWord(t *testing.T) {
	first, last, err := splitName("  Влад  ")
	if err != nil {
		t.Fatal(err)
	}
	if first != "Влад" || last != "" {
		t.Fatalf("splitName returned (%q, %q), want (%q, %q)", first, last, "Влад", "")
	}
}

func TestSplitNameRejectsBlankValue(t *testing.T) {
	if _, _, err := splitName(" \t "); err == nil {
		t.Fatal("splitName accepted a blank value")
	}
}
