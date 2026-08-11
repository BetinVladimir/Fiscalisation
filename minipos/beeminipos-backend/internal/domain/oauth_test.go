package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestClientCredentialsCachesAndSingleFlights(t *testing.T) {
	var requests atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		id, secret, ok := r.BasicAuth()
		if !ok || id != "minipos" || secret != "secret-secret-secret" {
			t.Error("invalid client authentication")
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("scope") != "fiscal.base" || r.Form.Get("audience") != "beefiscal" {
			t.Error("invalid token request", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "token_type": "Bearer", "expires_in": 3600})
	}))
	defer tokenServer.Close()
	p := NewClientCredentialsProvider(tokenServer.URL, "minipos", "secret-secret-secret", "fiscal.base", "beefiscal", nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := p.Token(context.Background())
			if err != nil || token != "token-1" {
				t.Errorf("token=%q err=%v", token, err)
			}
		}()
	}
	wg.Wait()
	if requests.Load() != 1 {
		t.Fatalf("token endpoint requests=%d", requests.Load())
	}
}

func TestFiscalCallRefreshesOnceAfterUnauthorized(t *testing.T) {
	var tokenRequests, apiRequests atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := tokenRequests.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-" + string(rune('0'+n)), "token_type": "Bearer", "expires_in": 3600})
	}))
	defer tokenServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := apiRequests.Add(1)
		if r.Header.Get("Idempotency-Key") != "same-key" {
			t.Error("idempotency key changed")
		}
		if n == 1 {
			if r.Header.Get("Authorization") != "Bearer token-1" {
				t.Error("wrong initial token")
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token-2" {
			t.Error("token was not refreshed")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer apiServer.Close()
	s := NewService(apiServer.URL, "2026-08-07")
	s.SetFiscalAuthProvider(NewClientCredentialsProvider(tokenServer.URL, "id", "secret-secret-secret", "fiscal.base", "", nil))
	var out map[string]any
	if err := s.call(http.MethodPost, "/sales", "same-key", map[string]any{"value": 1}, &out); err != nil {
		t.Fatal(err)
	}
	if tokenRequests.Load() != 2 || apiRequests.Load() != 2 || out["ok"] != true {
		t.Fatal(tokenRequests.Load(), apiRequests.Load(), out)
	}
}

func TestClientCredentialsRejectsUnsafeTokenResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "token_type": "MAC", "expires_in": 3600})
	}))
	defer server.Close()
	p := NewClientCredentialsProvider(server.URL, "id", "secret-secret-secret", "fiscal.base", "", nil)
	if _, err := p.Token(context.Background()); err == nil {
		t.Fatal("non-Bearer token accepted")
	}
}
