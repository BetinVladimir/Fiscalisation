package integration

import "testing"

func TestNormalizeTaxIsCountryAgnostic(t *testing.T) {
	c, k, v, err := normalizeTax(" bg ", " eik ", "123.456.789")
	if err != nil || c != "BG" || k != "EIK" || v != "123456789" {
		t.Fatalf("unexpected normalization: %q %q %q %v", c, k, v, err)
	}
	if _, _, _, err = normalizeTax("BG", "EIK", "ABCDEFGHI"); err == nil {
		t.Fatal("accepted non-numeric BG EIK")
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

func TestResourceSchemasRejectUnknownAndMissingFields(t *testing.T) {
	if e := validateResourcePayload("location", []byte(`{"name":"Store","address":"Street"}`)); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{"name":"Store"}`, `{"name":"Store","address":"Street","tenant_id":"forged"}`} {
		if validateResourcePayload("location", []byte(raw)) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
