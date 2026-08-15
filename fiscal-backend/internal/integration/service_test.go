package integration

import "testing"

func TestNormalizeTaxIsCountryAgnostic(t *testing.T) {
	c, k, v, err := normalizeTax(" bg ", " vat ", "BG-123.456/789")
	if err != nil || c != "BG" || k != "VAT" || v != "BG123456789" {
		t.Fatalf("unexpected normalization: %q %q %q %v", c, k, v, err)
	}
}

func TestWebhookValidationRejectsLocalTargets(t *testing.T) {
	for _, raw := range []string{"http://example.com/hook", "https://localhost/hook", "https://127.0.0.1/hook", "https://user@example.com/hook"} {
		if validateSystemWebhook(raw, []string{"integration.command.updated"}) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if err := validateSystemWebhook("https://hooks.example.com/beefiscal", []string{"integration.command.updated"}); err != nil {
		t.Fatal(err)
	}
}

func TestOpaqueTokenRoundTrip(t *testing.T) {
	raw, err := token("tenant_live", "credential-id")
	if err != nil {
		t.Fatal(err)
	}
	if id, ok := splitToken(raw, "tenant_live"); !ok || id != "credential-id" {
		t.Fatalf("bad token split: %q %v", id, ok)
	}
}
