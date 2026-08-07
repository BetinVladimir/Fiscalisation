package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type Claims struct {
	Subject   string   `json:"sub"`
	TenantID  string   `json:"tenant_id"`
	Roles     []string `json:"roles"`
	Scope     string   `json:"scope"`
	ExpiresAt int64    `json:"exp"`
}
type key struct{}

func ClaimsFrom(ctx context.Context) (Claims, bool) { v, ok := ctx.Value(key{}).(Claims); return v, ok }
func Middleware(secret string, next http.Handler) http.Handler {
	return MiddlewareWithOIDC(secret, nil, next)
}
func MiddlewareWithOIDC(secret string, oidc *OIDCVerifier, next http.Handler) http.Handler {
	if secret == "" && oidc == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/public/v1") || strings.HasSuffix(r.URL.Path, "/fiscal-webhooks") {
			next.ServeHTTP(w, r)
			return
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		var c Claims
		var e error
		if oidc != nil {
			c, e = oidc.Parse(raw, time.Now())
		} else {
			c, e = Parse(raw, []byte(secret), time.Now())
		}
		if e != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !hasScope(c, "fiscal.base") || !Allowed(c, r.Method, r.URL.Path) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, c)))
	})
}

func hasScope(c Claims, required string) bool {
	for _, scope := range strings.Fields(c.Scope) {
		if scope == required {
			return true
		}
	}
	return false
}

func hasAny(c Claims, allowed ...string) bool {
	for _, actual := range c.Roles {
		for _, expected := range allowed {
			if strings.EqualFold(actual, expected) {
				return true
			}
		}
	}
	return false
}

func Allowed(c Claims, method, path string) bool {
	if method == http.MethodGet {
		return hasAny(c, "CASHIER", "SUPERVISOR", "ADMIN", "AUDITOR")
	}
	if path == "/public/v1/minipos/configuration" || strings.Contains(path, "/products") || strings.Contains(path, "/employees") {
		return hasAny(c, "SUPERVISOR", "ADMIN")
	}
	return hasAny(c, "CASHIER", "SUPERVISOR", "ADMIN")
}
func Parse(raw string, secret []byte, now time.Time) (Claims, error) {
	var c Claims
	p := strings.Split(raw, ".")
	if len(p) != 3 {
		return c, errors.New("invalid token")
	}
	head, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil {
		return c, e
	}
	var h struct {
		Alg string `json:"alg"`
	}
	if json.Unmarshal(head, &h) != nil || h.Alg != "HS256" {
		return c, errors.New("invalid algorithm")
	}
	sig, e := base64.RawURLEncoding.DecodeString(p[2])
	if e != nil {
		return c, e
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(p[0] + "." + p[1]))
	if !hmac.Equal(sig, m.Sum(nil)) {
		return c, errors.New("invalid signature")
	}
	b, e := base64.RawURLEncoding.DecodeString(p[1])
	if e != nil || json.Unmarshal(b, &c) != nil {
		return c, errors.New("invalid claims")
	}
	if err := validateClaims(c, now); err != nil {
		return c, err
	}
	return c, nil
}
func validateClaims(c Claims, now time.Time) error {
	if c.Subject == "" || c.TenantID == "" || len(c.Roles) == 0 || c.ExpiresAt <= now.Unix() {
		return errors.New("expired or incomplete")
	}
	for _, role := range c.Roles {
		if !hasAny(Claims{Roles: []string{role}}, "CASHIER", "SUPERVISOR", "ADMIN", "AUDITOR") {
			return errors.New("unknown role")
		}
	}
	return nil
}
