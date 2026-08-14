package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" minipos/+/events, , fiscal/results/# ")
	want := []string{"minipos/+/events", "fiscal/results/#"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCSV() = %#v, want %#v", got, want)
	}
}

func TestProdGuards(t *testing.T) {
	base := Config{AppEnv: "prod", DatabaseURL: "postgres://writer@db/minipos", RLSDatabaseURL: "postgres://reader@db/minipos", FiscalBaseURL: "https://fiscal.example/public/v1", CORSAllowedOrigins: "https://pos.example", WebhookVerificationKey: strings.Repeat("w", 32), OIDCIssuer: "https://id.example", OIDCAudience: "beeminipos", OIDCJWKSURL: "https://id.example/jwks", OAuthTokenURL: "https://id.example/token", OAuthClientID: "minipos", OAuthClientSecret: "production-secret-32-bytes", OAuthScope: "fiscal.base", LocalTokenSigningKeyPEM: "configured-by-secret-store", LocalTokenSigningKID: "local-2026-01", LocalTokenIssuer: "https://pos.example"}
	if e := base.Validate(); e != nil {
		t.Fatal(e)
	}
	x := base
	x.WebhookVerificationKey = "too-short"
	if x.Validate() == nil {
		t.Fatal("weak webhook verification key accepted")
	}
	x = base
	x.FiscalBaseURL = "http://fiscal.example/public/v1"
	if x.Validate() == nil {
		t.Fatal("insecure Fiscal public API accepted")
	}
	x = base
	x.CORSAllowedOrigins = "*"
	if x.Validate() == nil {
		t.Fatal("wildcard CORS accepted")
	}
	x = base
	x.CORSAllowedOrigins = "https://pos.example/path"
	if x.Validate() == nil {
		t.Fatal("non-origin CORS URL accepted")
	}
	x = base
	x.OAuthClientSecret = ""
	if x.Validate() == nil {
		t.Fatal("missing OAuth secret accepted")
	}
	x = base
	x.FiscalAuthToken = "static"
	if x.Validate() == nil {
		t.Fatal("static production token accepted")
	}
	x = base
	x.AuthHMACKey = "dev"
	if x.Validate() == nil {
		t.Fatal("HMAC auth accepted in PROD")
	}
	x = base
	x.OIDCIssuer = ""
	if x.Validate() == nil {
		t.Fatal("missing OIDC accepted")
	}
	x = base
	x.OIDCIssuer = "http://id.example"
	if x.Validate() == nil {
		t.Fatal("insecure OIDC issuer accepted")
	}
	x = base
	x.FiscalBaseURL = "postgres://fiscal"
	if x.Validate() == nil {
		t.Fatal("private DB coupling accepted")
	}
	x = base
	x.RLSDatabaseURL = ""
	if x.Validate() == nil {
		t.Fatal("missing RLS db accepted")
	}
	x = base
	x.RLSDatabaseURL = x.DatabaseURL
	if x.Validate() == nil {
		t.Fatal("shared writer/reader identity accepted")
	}
}

func TestFiscalBoundaryRequiresExactPublicAPIBaseInEveryEnvironment(t *testing.T) {
	t.Setenv("FISCAL_DATABASE_URL", "")
	for _, raw := range []string{
		"postgres://fiscal/db",
		"file:///tmp/fiscal.db",
		"http://fiscal.example/internal/v1",
		"http://fiscal.example/public/v1/",
		"http://user:secret@fiscal.example/public/v1",
		"http://fiscal.example/public/v1?tenant=other",
		"http://fiscal.example/public/v1#fragment",
		"not-a-url",
	} {
		c := Config{AppEnv: "dev", FiscalBaseURL: raw}
		if err := c.Validate(); err == nil {
			t.Fatalf("non-public Fiscal boundary accepted: %q", raw)
		}
	}
	for _, raw := range []string{"http://localhost:8080/public/v1", "https://fiscal.example/public/v1"} {
		c := Config{AppEnv: "dev", FiscalBaseURL: raw}
		if err := c.Validate(); err != nil {
			t.Fatalf("valid public Fiscal base rejected: %q: %v", raw, err)
		}
	}
}
