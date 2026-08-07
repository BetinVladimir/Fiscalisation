package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type rotatingJWKS struct {
	mu  sync.RWMutex
	kid string
	key *rsa.PrivateKey
}

func (r *rotatingJWKS) set(kid string, key *rsa.PrivateKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kid, r.key = kid, key
}
func (r *rotatingJWKS) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e := big.NewInt(int64(r.key.PublicKey.E)).Bytes()
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": r.kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(r.key.PublicKey.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(e)}}})
}
func oidcToken(t *testing.T, key *rsa.PrivateKey, kid, issuer string, audience any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": kid, "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"sub": "operator-1", "tenant_id": "tenant-1", "roles": []string{"CASHIER"}, "scope": "fiscal.base", "exp": time.Now().Add(time.Hour).Unix(), "iss": issuer, "aud": audience})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	h := crypto.SHA256.New()
	h.Write([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h.Sum(nil))
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func TestOIDCVerifierValidatesAndRefreshesRotatedKid(t *testing.T) {
	first, _ := rsa.GenerateKey(rand.Reader, 2048)
	second, _ := rsa.GenerateKey(rand.Reader, 2048)
	keys := &rotatingJWKS{}
	keys.set("first", first)
	server := httptest.NewServer(keys)
	defer server.Close()
	verifier := NewOIDCVerifier("https://issuer.example/", "beeminipos", server.URL)
	if claims, err := verifier.Parse(oidcToken(t, first, "first", "https://issuer.example", []string{"other", "beeminipos"}), time.Now()); err != nil || claims.TenantID != "tenant-1" {
		t.Fatalf("valid token rejected: %#v %v", claims, err)
	}
	keys.set("second", second)
	if _, err := verifier.Parse(oidcToken(t, second, "second", "https://issuer.example", "beeminipos"), time.Now().Add(6*time.Second)); err != nil {
		t.Fatalf("rotated kid rejected: %v", err)
	}
}
func TestOIDCVerifierRejectsWrongTrustClaimsAndSignature(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	attacker, _ := rsa.GenerateKey(rand.Reader, 2048)
	keys := &rotatingJWKS{}
	keys.set("trusted", key)
	server := httptest.NewServer(keys)
	defer server.Close()
	verifier := NewOIDCVerifier("https://issuer.example", "beeminipos", server.URL)
	for name, token := range map[string]string{"issuer": oidcToken(t, key, "trusted", "https://evil.example", "beeminipos"), "audience": oidcToken(t, key, "trusted", "https://issuer.example", "other"), "signature": oidcToken(t, attacker, "trusted", "https://issuer.example", "beeminipos")} {
		t.Run(name, func(t *testing.T) {
			if _, err := verifier.Parse(token, time.Now()); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
}
