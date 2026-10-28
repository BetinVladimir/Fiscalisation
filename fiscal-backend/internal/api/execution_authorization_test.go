package api

import (
	"bytes"
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecutionAuthorizationIdentityMappingRequiresAdmin(t *testing.T) {
	repo := domain.NewMemoryRepository()
	if err := repo.PutResource(domain.ResourceRecord{ID: "operator", Kind: "operator", TenantID: "tenant-a", Version: 1, Data: map[string]any{"code": "A001"}}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(domain.NewService(repo, domain.NewSimulator(true)), config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"})
	call := func(role, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+jwtSubject("tenant-a", "admin-subject", role))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"operator_id":"operator","actor_subject":"cashier-subject","expected_version":1}`
	if w := call("CASHIER", "/public/v1/operators:bind-identity", body, "identity-cashier-0001"); w.Code != 403 {
		t.Fatalf("cashier %d %s", w.Code, w.Body.String())
	}
	if w := call("ADMIN", "/public/v1/operators:bind-identity", body, "identity-admin-0001"); w.Code != 204 {
		t.Fatalf("admin %d %s", w.Code, w.Body.String())
	}
	if w := call("ADMIN", "/public/v1/operators:bind-identity", body, "identity-admin-0001"); w.Code != 204 || w.Body.Len() != 0 || w.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("empty replay %d %s", w.Code, w.Body.String())
	}
	op, _ := repo.Resource("operator", "operator")
	if op.Data["actor_subject"] != "cashier-subject" || op.Version != 2 {
		t.Fatalf("mapping %#v", op)
	}
	if w := call("CASHIER", "/public/v1/execution-authorizations", `{"workstation_session_id":"other","client_signing_public_key_der_base64":"invalid","tenant_id":"foreign"}`, "auth-unknown-00001"); w.Code != 400 {
		t.Fatalf("untrusted field %d %s", w.Code, w.Body.String())
	}
	if w := call("CASHIER", "/public/v1/execution-authorizations", `{"workstation_session_id":"other","client_signing_public_key_der_base64":"invalid"}`, "auth-foreign-00001"); w.Code != 409 {
		t.Fatalf("foreign session %d %s", w.Code, w.Body.String())
	}
}
