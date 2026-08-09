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
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	p := base64.RawURLEncoding.EncodeToString([]byte(payload))
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(h + "." + p))
	return h + "." + p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func TestParse(t *testing.T) {
	raw := token(`{"sub":"u","tenant_id":"t","roles":["cashier"],"scope":"fiscal.base","exp":2000}`, "secret")
	if c, e := Parse(raw, []byte("secret"), time.Unix(1000, 0)); e != nil || c.TenantID != "t" {
		t.Fatalf("%+v %v", c, e)
	}
	if _, e := Parse(raw, []byte("bad"), time.Unix(1000, 0)); e == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestRBACLeastPrivilege(t *testing.T) {
	cashier := Claims{Roles: []string{"CASHIER"}}
	supervisor := Claims{Roles: []string{"SUPERVISOR"}}
	admin := Claims{Roles: []string{"ADMIN"}}
	auditor := Claims{Roles: []string{"AUDITOR"}}
	if !Allowed(cashier, "POST", "/public/v1/minipos/orders") {
		t.Fatal("cashier cannot sell")
	}
	if Allowed(cashier, "POST", "/public/v1/minipos/products") {
		t.Fatal("cashier can edit catalog")
	}
	if Allowed(cashier, "GET", "/public/v1/minipos/employees") || Allowed(cashier, "GET", "/public/v1/minipos/reports/sales") {
		t.Fatal("cashier can read employee directory or tenant-wide report")
	}
	if !Allowed(cashier, "GET", "/public/v1/minipos/products") || !Allowed(cashier, "GET", "/public/v1/minipos/orders") {
		t.Fatal("cashier cannot read required POS resources")
	}
	if !Allowed(supervisor, "PATCH", "/public/v1/minipos/configuration") {
		t.Fatal("supervisor cannot configure")
	}
	if !Allowed(admin, "POST", "/public/v1/minipos/employees") {
		t.Fatal("admin cannot edit employees")
	}
	if !Allowed(auditor, "GET", "/public/v1/minipos/reports/sales") {
		t.Fatal("auditor cannot read")
	}
	if Allowed(auditor, "POST", "/public/v1/minipos/orders") {
		t.Fatal("auditor can sell")
	}
}

func TestParseRejectsUnknownRole(t *testing.T) {
	raw := token(`{"sub":"u1","tenant_id":"t1","roles":["ROOT"],"scope":"fiscal.base","exp":2000}`, "secret")
	if _, err := Parse(raw, []byte("secret"), time.Unix(1000, 0)); err == nil {
		t.Fatal("unknown role accepted")
	}
}

func TestRequiredOAuthScope(t *testing.T) {
	if !hasScope(Claims{Scope: "fiscal.base"}, "fiscal.base") {
		t.Fatal("required scope rejected")
	}
	if hasScope(Claims{Scope: "openid"}, "fiscal.base") {
		t.Fatal("missing scope accepted")
	}
}

func TestMiddlewareRejectsMissingScope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	raw := token(`{"sub":"u1","tenant_id":"t1","roles":["CASHIER"],"scope":"openid","exp":4102444800}`, "secret")
	r := httptest.NewRequest(http.MethodPost, "/public/v1/minipos/orders", nil)
	r.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	Middleware("secret", next).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("missing scope status=%d", w.Code)
	}
}

func TestMiddlewareRequiresExplicitBearerAndExactWebhookExemption(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	raw := token(`{"sub":"u1","tenant_id":"t1","roles":["CASHIER"],"scope":"fiscal.base","exp":4102444800}`, "secret")
	for _, header := range []string{raw, "Basic " + raw, "Bearer", "Bearer " + raw + " extra"} {
		r := httptest.NewRequest(http.MethodPost, "/public/v1/minipos/orders", nil)
		r.Header.Set("Authorization", header)
		w := httptest.NewRecorder()
		Middleware("secret", next).ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("malformed authorization accepted: %q status=%d", header, w.Code)
		}
	}
	for path, want := range map[string]int{"/public/v1/fiscal-webhooks": http.StatusNoContent, "/public/v1/evil/fiscal-webhooks": http.StatusUnauthorized} {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		Middleware("secret", next).ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("webhook exemption path=%s status=%d want=%d", path, w.Code, want)
		}
	}
}
