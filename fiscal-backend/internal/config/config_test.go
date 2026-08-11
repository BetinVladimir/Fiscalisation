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
	base := Config{AppEnv: "prod", PublicBaseURL: "https://fiscal.example/public/v1", CORSAllowedOrigins: "https://admin.example,https://pos.example", DatabaseURL: "postgres://writer@db/fiscal", RLSDatabaseURL: "postgres://reader@db/fiscal", WebhookSigningKey: strings.Repeat("w", 32), OIDCIssuer: "https://id.example", OIDCAudience: "beefiscal", OIDCJWKSURL: "https://id.example/jwks", BLESigningKey: strings.Repeat("b", 32), DeviceCACertFile: "/run/secrets/device-ca.crt", DeviceCAKeyFile: "/run/secrets/device-ca.key", DeviceMQTTTLSURI: "ssl://mqtt.example:8883"}
	if e := base.Validate(); e != nil {
		t.Fatal(e)
	}
	x := base
	x.WebhookSigningKey = "too-short"
	if x.Validate() == nil {
		t.Fatal("weak webhook signing key accepted")
	}
	x = base
	x.PublicBaseURL = "http://fiscal.example/public/v1"
	if x.Validate() == nil {
		t.Fatal("insecure public base URL accepted")
	}
	x = base
	x.CORSAllowedOrigins = "*"
	if x.Validate() == nil {
		t.Fatal("wildcard CORS accepted")
	}
	x = base
	x.CORSAllowedOrigins = "https://admin.example/path"
	if x.Validate() == nil {
		t.Fatal("non-origin CORS URL accepted")
	}
	x = base
	x.AllowStubAdapters = true
	if x.Validate() == nil {
		t.Fatal("stub accepted")
	}
	x = base
	x.SimulatorCardTerminalAvailable = true
	if x.Validate() == nil {
		t.Fatal("simulated card terminal accepted in PROD")
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
