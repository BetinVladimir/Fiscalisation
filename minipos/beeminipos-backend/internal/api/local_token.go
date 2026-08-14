package api

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fiscalisation/beeminipos-backend/internal/auth"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type localTokenRequest struct {
	AdapterDeviceID   string `json:"adapter_device_id"`
	LocationID        string `json:"location_id"`
	RegisterID        string `json:"register_id"`
	OperatorID        string `json:"operator_id"`
	ShiftID           string `json:"shift_id"`
	BindingGeneration int64  `json:"binding_generation"`
	AdapterBaseURL    string `json:"adapter_base_url"`
}

func (h *handler) fiscalRouteHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		problem(w, 405, "method")
		return
	}
	started := time.Now()
	if err := h.s.FiscalPing(); err != nil {
		problem(w, 503, "fiscal cloud route unavailable")
		return
	}
	write(w, 200, map[string]any{"status": "ok", "latency_ms": time.Since(started).Milliseconds()})
}

func parseES256Private(raw string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("LOCAL_TOKEN_SIGNER_UNAVAILABLE")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if ec, ok := key.(*ecdsa.PrivateKey); ok && ec.Curve.Params().Name == "P-256" {
			return ec, nil
		}
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil && key.Curve.Params().Name == "P-256" {
		return key, nil
	}
	return nil, errors.New("LOCAL_TOKEN_SIGNER_INVALID")
}
func p1363(r, s *big.Int) []byte {
	out := make([]byte, 64)
	r.FillBytes(out[:32])
	s.FillBytes(out[32:])
	return out
}
func newRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "unavailable"
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
func (h *handler) localFiscalToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "method")
		return
	}
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok || claims.Subject == "" || claims.TenantID == "" {
		problem(w, 401, "authenticated operator required")
		return
	}
	var in localTokenRequest
	if !decode(w, r, &in) {
		return
	}
	adapterURL, urlErr := url.Parse(in.AdapterBaseURL)
	if in.AdapterDeviceID == "" || in.LocationID == "" || in.RegisterID == "" || in.OperatorID == "" || in.ShiftID == "" || in.BindingGeneration < 1 || urlErr != nil || adapterURL.Scheme != "http" || adapterURL.Host == "" || adapterURL.User != nil || adapterURL.RawQuery != "" || adapterURL.Fragment != "" || !strings.HasSuffix(strings.TrimSuffix(adapterURL.Path, "/"), "/beeloy/local/v1") {
		problem(w, 422, "invalid local fiscal token request")
		return
	}
	shift, err := h.s.ShiftForTenant(in.ShiftID, claims.TenantID)
	if err != nil || shift.State != "OPEN" || shift.RegisterID != in.RegisterID || shift.EmployeeID != in.OperatorID || !h.s.IdentityMatchesEmployee(claims.TenantID, claims.Issuer, claims.Subject, in.OperatorID) {
		problem(w, 403, "operator, open shift and register binding required")
		return
	}
	configuration, err := h.s.ConfigurationFor(claims.TenantID)
	if err != nil || configuration.LocationID != in.LocationID || configuration.FiscalRegisterID != in.RegisterID || configuration.FiscalAdapterID != in.AdapterDeviceID || configuration.BindingGeneration != in.BindingGeneration || strings.TrimSuffix(configuration.AdapterBaseURL, "/") != strings.TrimSuffix(in.AdapterBaseURL, "/") {
		problem(w, 409, "active fiscal adapter association changed")
		return
	}
	key, err := parseES256Private(h.c.LocalTokenSigningKeyPEM)
	if err != nil {
		problem(w, 503, err.Error())
		return
	}
	now := time.Now().UTC()
	expires := now.Add(15 * time.Minute)
	header, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "JWT", "kid": h.c.LocalTokenSigningKID})
	body, _ := json.Marshal(map[string]any{"iss": h.c.LocalTokenIssuer, "aud": []string{"beeloy-local-fiscal-adapter", in.AdapterDeviceID}, "sub": claims.Subject, "jti": newRequestID(), "iat": now.Unix(), "nbf": now.Add(-5 * time.Second).Unix(), "exp": expires.Unix(), "tenant_id": claims.TenantID, "location_id": in.LocationID, "register_id": in.RegisterID, "operator_id": in.OperatorID, "shift_id": in.ShiftID, "adapter_device_id": in.AdapterDeviceID, "binding_generation": in.BindingGeneration, "scope": "fiscal.execute fiscal.read fiscal.printer_test"})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(unsigned))
	rr, ss, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		problem(w, 503, "local token signing failed")
		return
	}
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(p1363(rr, ss))
	write(w, 201, map[string]any{"access_token": token, "token_type": "Bearer", "expires_at": expires, "adapter_base_url": in.AdapterBaseURL, "kid": h.c.LocalTokenSigningKID})
}
