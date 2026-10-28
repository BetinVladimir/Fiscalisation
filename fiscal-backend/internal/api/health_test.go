package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
)

// unhealthyPingRepo embeds MemoryRepository and overrides PingContext to
// simulate a persistent-storage outage.
type unhealthyPingRepo struct {
	*domain.MemoryRepository
}

func (r *unhealthyPingRepo) PingContext(_ context.Context) error {
	return errors.New("connection refused")
}

// TestReadyzFailsWhenRepositoryUnavailable verifies that GET /readyz returns
// 503 when the repository's PingContext reports an outage, while GET /livez
// (process liveness) continues to return 200.
func TestReadyzFailsWhenRepositoryUnavailable(t *testing.T) {
	repo := &unhealthyPingRepo{domain.NewMemoryRepository()}
	svc := domain.NewService(repo, nil)
	h := NewHandler(svc, config.Config{
		APIVersion:  "2026-08-07",
		AuthHMACKey: "01234567890123456789012345678901",
	})

	// /readyz must return 503 when persistence is unavailable.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz = %d, want 503", w.Code)
	}

	// /livez must still return 200 (process is alive).
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("GET /livez = %d, want 200", w2.Code)
	}
}

// TestReadyzOKWhenRepositoryHealthy verifies that after recovery (healthy repo)
// GET /readyz returns 200 without a process restart.
func TestReadyzOKWhenRepositoryHealthy(t *testing.T) {
	svc := domain.NewService(domain.NewMemoryRepository(), nil)
	h := NewHandler(svc, config.Config{
		APIVersion:  "2026-08-07",
		AuthHMACKey: "01234567890123456789012345678901",
	})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /readyz = %d, want 200", w.Code)
	}
}
