package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
)

func TestConnectivityPingIsBodylessAndDoesNotRequireBusinessDependencies(t *testing.T) {
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), nil), config.Config{APIVersion: "2026-08-07"})
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		r := httptest.NewRequest(method, "/connectivity/ping", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
			t.Fatalf("%s ping = %d body=%q", method, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-BeeFiscal-Ping") != "1" {
			t.Fatalf("%s ping headers missing: %#v", method, w.Header())
		}
	}
}

func TestConnectivityPingRejectsMutationMethods(t *testing.T) {
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), nil), config.Config{APIVersion: "2026-08-07"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/connectivity/ping", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST ping = %d", w.Code)
	}
}

func TestConnectivityPingBypassesBusinessAuthenticationAndAPIVersion(t *testing.T) {
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), nil), config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/connectivity/ping", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("unauthenticated ping = %d", w.Code)
	}
}
