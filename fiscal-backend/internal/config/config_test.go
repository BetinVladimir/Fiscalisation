package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" fiscal/+/events, , edge/commands/# ")
	want := []string{"fiscal/+/events", "edge/commands/#"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCSV() = %#v, want %#v", got, want)
	}
}

func TestProdGuards(t *testing.T) {
	base := Config{AppEnv: "prod", DatabaseURL: "postgres://db", WebhookSigningKey: "production-webhook", OIDCIssuer: "https://id.example", OIDCAudience: "beefiscal", OIDCJWKSURL: "https://id.example/jwks", BLESigningKey: strings.Repeat("b", 32)}
	if e := base.Validate(); e != nil {
		t.Fatal(e)
	}
	x := base
	x.AllowStubAdapters = true
	if x.Validate() == nil {
		t.Fatal("stub accepted")
	}
	x = base
	x.AuthHMACKey = "dev-key"
	if x.Validate() == nil {
		t.Fatal("HMAC auth accepted in PROD")
	}
	x = base
	x.OIDCJWKSURL = ""
	if x.Validate() == nil {
		t.Fatal("missing OIDC accepted")
	}
	x = base
	x.OIDCJWKSURL = "http://id.example/jwks"
	if x.Validate() == nil {
		t.Fatal("insecure JWKS URL accepted")
	}
	x = base
	x.DatabaseURL = ""
	if x.Validate() == nil {
		t.Fatal("missing db accepted")
	}
}
