package api

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"time"
)

type executionAuthorizationResponse struct {
	fiscalintegration.ExecutionAuthorizationResponse
	TransportToken          string    `json:"transport_token"`
	TransportTokenExpiresAt time.Time `json:"transport_token_expires_at"`
}

func (h *handler) executionTransportToken(envelope fiscalintegration.ExecutionAuthorizationEnvelope, source auth.Claims, shiftID string) (string, time.Time, error) {
	raw, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
	var v struct {
		Tenant     string `json:"tenant_id"`
		Location   string `json:"location_id"`
		Register   string `json:"register_id"`
		Operator   string `json:"operator_id"`
		Edge       string `json:"edge_device_id"`
		Generation int64  `json:"binding_generation"`
		NBF        int64  `json:"nbf"`
		Exp        int64  `json:"exp"`
	}
	now := time.Now().UTC()
	if err != nil || json.Unmarshal(raw, &v) != nil || v.Tenant == "" || v.Location == "" || v.Register == "" || v.Operator == "" || v.Edge == "" || v.Generation < 1 || now.Unix() < v.NBF || now.Unix() >= v.Exp {
		return "", time.Time{}, errors.New("trusted root authorization claims required")
	}
	key, err := parseES256Private(h.c.LocalTokenSigningKeyPEM)
	if err != nil || h.c.LocalTokenIssuer == "" || h.c.LocalTokenSigningKID == "" {
		return "", time.Time{}, errors.New("LOCAL_TOKEN_SIGNER_UNAVAILABLE")
	}
	expiry := v.Exp
	if source.ExpiresAt < expiry {
		expiry = source.ExpiresAt
	}
	header, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "JWT", "kid": h.c.LocalTokenSigningKID})
	body, _ := json.Marshal(map[string]any{"iss": h.c.LocalTokenIssuer, "aud": []string{"beeloy-local-fiscal-adapter", v.Edge}, "sub": source.Subject, "jti": newRequestID(), "iat": now.Unix(), "nbf": now.Add(-5 * time.Second).Unix(), "exp": expiry, "tenant_id": v.Tenant, "location_id": v.Location, "register_id": v.Register, "operator_id": v.Operator, "shift_id": shiftID, "adapter_device_id": v.Edge, "binding_generation": v.Generation, "scope": "fiscal.execute fiscal.read"})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", time.Time{}, err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(p1363(r, s)), time.Unix(expiry, 0).UTC(), nil
}
