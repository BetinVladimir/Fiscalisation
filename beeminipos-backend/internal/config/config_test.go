package config

import (
	"reflect"
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
	base := Config{AppEnv: "prod", DatabaseURL: "postgres://writer@db/minipos", RLSDatabaseURL: "postgres://reader@db/minipos", FiscalBaseURL: "https://fiscal/public/v1", WebhookVerificationKey: "production-webhook", OIDCIssuer: "https://id.example", OIDCAudience: "beeminipos", OIDCJWKSURL: "https://id.example/jwks", OAuthTokenURL: "https://id.example/token", OAuthClientID: "minipos", OAuthClientSecret: "production-secret-32-bytes", OAuthScope: "fiscal.base"}
	if e := base.Validate(); e != nil {
		t.Fatal(e)
	}
	x := base
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
