package api

import (
	"bytes"
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReconciliationAdministratorRootBoundary(t *testing.T) {
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"})
	for _, tc := range []struct {
		role, body string
		want       int
	}{{"CASHIER", `{}`, 403}, {"SUPERVISOR", `{}`, 403}, {"ADMIN", `{} {}`, 400}, {"ADMIN", `{"administrator":true}`, 400}} {
		r := httptest.NewRequest(http.MethodPost, "/public/v1/payment-reconciliations:bind-administrator", bytes.NewBufferString(tc.body))
		r.Header.Set("Authorization", "Bearer "+jwtSubject("tenant-a", "administrator", tc.role))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Idempotency-Key", "administrator-binding-0001")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
