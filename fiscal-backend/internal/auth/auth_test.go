package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func token(payload, key string) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	p := base64.RawURLEncoding.EncodeToString([]byte(payload))
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(h + "." + p))
	return h + "." + p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func TestParse(t *testing.T) {
	now := time.Unix(1000, 0)
	raw := token(`{"sub":"u1","tenant_id":"t1","roles":["cashier"],"scope":"fiscal.base","exp":2000}`, "secret")
	c, e := Parse(raw, []byte("secret"), now)
	if e != nil || c.TenantID != "t1" {
		t.Fatalf("%+v %v", c, e)
	}
	if _, e = Parse(raw+"x", []byte("secret"), now); e == nil {
		t.Fatal("tampering accepted")
	}
	if _, e = Parse(raw, []byte("secret"), time.Unix(3000, 0)); e == nil {
		t.Fatal("expired accepted")
	}
}

func TestRBACLeastPrivilege(t *testing.T) {
	cashier := Claims{Roles: []string{"CASHIER"}}
	supervisor := Claims{Roles: []string{"SUPERVISOR"}}
	admin := Claims{Roles: []string{"ADMIN"}}
	auditor := Claims{Roles: []string{"AUDITOR"}}
	service := Claims{Roles: []string{"SERVICE"}}
	tests := []struct {
		name         string
		claims       Claims
		method, path string
		want         bool
	}{
		{"cashier sale", cashier, "POST", "/public/v1/sales", true},
		{"cashier cannot reverse", cashier, "POST", "/public/v1/sales/id/reversals", false},
		{"supervisor report", supervisor, "POST", "/public/v1/registers/id/reports", true},
		{"auditor read", auditor, "GET", "/public/v1/audit-events", true},
		{"cashier cannot read audit", cashier, "GET", "/public/v1/audit-events", false},
		{"cashier cannot read operator directory", cashier, "GET", "/public/v1/operators", false},
		{"supervisor cannot read operator identity registry", supervisor, "GET", "/public/v1/operators", false},
		{"admin may read operator directory", admin, "GET", "/public/v1/operators", true},
		{"cashier cannot read tenant reports", cashier, "GET", "/public/v1/reports", false},
		{"supervisor may read tenant reports", supervisor, "GET", "/public/v1/reports", true},
		{"auditor may read exports", auditor, "GET", "/public/v1/exports/e1", true},
		{"cashier cannot read webhook registry", cashier, "GET", "/public/v1/webhook-endpoints", false},
		{"service may read provisioning state", service, "GET", "/public/v1/devices/d1/provisioning-sessions", true},
		{"cashier cannot read provisioning state", cashier, "GET", "/public/v1/devices/d1/provisioning-sessions", false},
		{"cashier may read final device readiness", cashier, "GET", "/public/v1/devices/d1/readiness", true},
		{"auditor cannot mutate", auditor, "POST", "/public/v1/sales", false},
		{"admin provisioning", admin, "POST", "/public/v1/devices/id/provisioning-sessions", true},
		{"cashier cannot administer", cashier, "POST", "/public/v1/devices", false},
		{"cashier cannot create register", cashier, "POST", "/public/v1/registers", false},
		{"cashier may request scoped BLE session", cashier, "POST", "/public/v1/registers/id/ble-sessions", true},
		{"cashier may refresh own-tenant BLE session", cashier, "POST", "/public/v1/ble-sessions/id/refresh", true},
		{"service cannot request operator BLE authority", service, "POST", "/public/v1/registers/id/ble-sessions", false},
		{"service diagnostics", service, "GET", "/public/v1/devices/id/diagnostics", true},
		{"auditor cannot diagnose", auditor, "GET", "/public/v1/devices/id/diagnostics", false},
		{"service edge sync", service, "POST", "/public/v1/edge-sync/batches", true},
		{"admin cannot impersonate edge", admin, "POST", "/public/v1/edge-sync/batches", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Allowed(tt.claims, tt.method, tt.path); got != tt.want {
				t.Fatalf("Allowed=%v want %v", got, tt.want)
			}
		})
	}
}

func TestParseRejectsUnknownRole(t *testing.T) {
	raw := token(`{"sub":"u1","tenant_id":"t1","roles":["ROOT"],"scope":"fiscal.base","exp":2000}`, "secret")
	if _, err := Parse(raw, []byte("secret"), time.Unix(1000, 0)); err == nil {
		t.Fatal("unknown role accepted")
	}
}

func TestRequiredOAuthScope(t *testing.T) {
	if !hasScope(Claims{Scope: "openid fiscal.base profile"}, "fiscal.base") {
		t.Fatal("required scope rejected")
	}
	if hasScope(Claims{Scope: "fiscal.read"}, "fiscal.base") {
		t.Fatal("missing scope accepted")
	}
}

func TestMiddlewareRejectsMissingScope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	raw := token(`{"sub":"u1","tenant_id":"t1","roles":["CASHIER"],"scope":"openid","exp":4102444800}`, "secret")
	r := httptest.NewRequest(http.MethodPost, "/public/v1/sales", nil)
	r.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	Middleware("secret", next).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("missing scope status=%d", w.Code)
	}
}

func TestMiddlewareRequiresExplicitBearerScheme(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	raw := token(`{"sub":"u1","tenant_id":"t1","roles":["CASHIER"],"scope":"fiscal.base","exp":4102444800}`, "secret")
	for _, header := range []string{raw, "Basic " + raw, "Bearer", "Bearer " + raw + " extra"} {
		r := httptest.NewRequest(http.MethodPost, "/public/v1/sales", nil)
		r.Header.Set("Authorization", header)
		w := httptest.NewRecorder()
		Middleware("secret", next).ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("malformed authorization accepted: %q status=%d", header, w.Code)
		}
	}
}
