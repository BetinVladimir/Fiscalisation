package api

import (
	"bytes"
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaymentReconciliationAdminBoundary(t *testing.T) {
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"})
	for index, tc := range []struct {
		role, body string
		want       int
	}{{"CASHIER", `{}`, 403}, {"SUPERVISOR", `{}`, 403}, {"ADMIN", `{"terminal_id":"caller-route"}`, 400}, {"ADMIN", `{} {}`, 400}, {"ADMIN", `{"operation_id":"missing","payment_id":"missing","observed_state":"CARD_APPROVED","evidence_reference":"case","evidence_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, 409}} {
		r := httptest.NewRequest(http.MethodPost, "/public/v1/payment-reconciliations:authorize", bytes.NewBufferString(tc.body))
		r.Header.Set("Authorization", "Bearer "+jwtSubject("tenant-a", "admin-subject", tc.role))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", fmt.Sprintf("reconcile-admin-key-%08d", index))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("role=%s body=%s got=%d %s", tc.role, tc.body, w.Code, w.Body.String())
		}
	}
}
