package auth

import (
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OIDCVerifier struct {
	issuer, audience, jwksURL string
	client                    *http.Client
	mu                        sync.RWMutex
	keys                      map[string]*rsa.PublicKey
	expires                   time.Time
	lastRefresh               time.Time
}

func NewOIDCVerifier(issuer, audience, jwksURL string) *OIDCVerifier {
	if issuer == "" || audience == "" || jwksURL == "" {
		return nil
	}
	return &OIDCVerifier{issuer: strings.TrimSuffix(issuer, "/"), audience: audience, jwksURL: jwksURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (v *OIDCVerifier) Parse(raw string, now time.Time) (Claims, error) {
	var claims Claims
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return claims, errors.New("invalid token")
	}
	var header struct{ Alg, Kid string }
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(decoded, &header) != nil || header.Alg != "RS256" || header.Kid == "" {
		return claims, errors.New("invalid oidc header")
	}
	key, err := v.key(header.Kid, now, false)
	if err != nil {
		key, err = v.key(header.Kid, now, true)
	}
	if err != nil {
		return claims, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return claims, errors.New("invalid oidc signature")
	}
	digest := crypto.SHA256.New()
	digest.Write([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest.Sum(nil), signature) != nil {
		return claims, errors.New("invalid oidc signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errors.New("invalid oidc claims")
	}
	var wire struct {
		Claims
		Issuer   string          `json:"iss"`
		Audience json.RawMessage `json:"aud"`
	}
	if json.Unmarshal(payload, &wire) != nil || strings.TrimSuffix(wire.Issuer, "/") != v.issuer || !audienceContains(wire.Audience, v.audience) {
		return claims, errors.New("invalid oidc claims")
	}
	claims = wire.Claims
	if err = validateClaims(claims, now); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

func audienceContains(raw json.RawMessage, expected string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == expected
	}
	var many []string
	if json.Unmarshal(raw, &many) != nil {
		return false
	}
	for _, candidate := range many {
		if candidate == expected {
			return true
		}
	}
	return false
}

func (v *OIDCVerifier) key(kid string, now time.Time, force bool) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	fresh := now.Before(v.expires)
	lastRefresh := v.lastRefresh
	v.mu.RUnlock()
	if ok && fresh && !force {
		return key, nil
	}
	if force && now.Sub(lastRefresh) < 5*time.Second {
		return nil, errors.New("unknown oidc key")
	}
	if err := v.refresh(now); err != nil {
		return nil, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, errors.New("unknown oidc key")
	}
	return key, nil
}

func (v *OIDCVerifier) refresh(now time.Time) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	response, err := v.client.Get(v.jwksURL)
	if err != nil {
		return errors.New("jwks unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("jwks unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return errors.New("invalid jwks")
	}
	var set struct {
		Keys []struct{ Kty, Kid, Use, Alg, N, E string }
	}
	if json.Unmarshal(body, &set) != nil {
		return errors.New("invalid jwks")
	}
	keys := map[string]*rsa.PublicKey{}
	for _, jwk := range set.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" || (jwk.Use != "" && jwk.Use != "sig") || (jwk.Alg != "" && jwk.Alg != "RS256") {
			continue
		}
		n, nerr := base64.RawURLEncoding.DecodeString(jwk.N)
		e, eerr := base64.RawURLEncoding.DecodeString(jwk.E)
		if nerr != nil || eerr != nil || len(n) < 128 || len(e) == 0 || len(e) > 4 {
			continue
		}
		exponent := 0
		for _, b := range e {
			exponent = exponent<<8 | int(b)
		}
		if exponent < 3 {
			continue
		}
		keys[jwk.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
	}
	if len(keys) == 0 {
		return errors.New("jwks has no usable keys")
	}
	v.keys, v.expires, v.lastRefresh = keys, now.Add(5*time.Minute), now
	return nil
}
