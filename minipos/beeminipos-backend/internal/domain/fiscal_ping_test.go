package domain

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFiscalPingAcceptsNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/connectivity/ping" {
			t.Fatalf("unexpected ping request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := NewService(server.URL, "2026-08-07").FiscalPing(); err != nil {
		t.Fatalf("204 ping must be accepted: %v", err)
	}
}
