package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	dispatchOrderID     = "00000000-0000-4000-8000-000000000031"
	dispatchOperationID = "00000000-0000-4000-8000-000000000032"
	dispatchSessionID   = "00000000-0000-4000-8000-000000000033"
)

type dispatchBridge struct{ err error }

func (s *dispatchBridge) ExecutionIdentity(context.Context, string, string, string) (fiscalintegration.ExecutionIdentityContext, error) {
	return fiscalintegration.ExecutionIdentityContext{}, nil
}
func (s *dispatchBridge) IssueExecutionAuthorization(context.Context, string, fiscalintegration.ExecutionAuthorizationRequest, string) (fiscalintegration.ExecutionAuthorizationResponse, error) {
	return fiscalintegration.ExecutionAuthorizationResponse{}, nil
}
func (s *dispatchBridge) DispatchExecutionAuthorization(_ context.Context, _, _ string) (fiscalintegration.FiscalOperation, error) {
	if s.err != nil {
		return fiscalintegration.FiscalOperation{}, s.err
	}
	return fiscalintegration.FiscalOperation{
		OperationID: dispatchOperationID, ClientOperationID: dispatchOperationID,
		ReceiptSessionID: dispatchSessionID, SaleID: "fiscal-sale", RegisterID: "fiscal-register",
		TenantID: "fiscal-tenant-a", State: "ACCEPTED",
	}, nil
}

const dispatchPath = "/public/v1/minipos/execution-authorizations:dispatch"

func makeDispatchHandler(t *testing.T, br *dispatchBridge) (http.Handler, string) {
	t.Helper()
	order := domain.Order{
		ID: dispatchOrderID, TenantID: "tenant-a", ShiftID: "disp-shift",
		RegisterID: "source-reg", FiscalRegisterID: "fiscal-register",
		CheckoutOperationID: dispatchOperationID, ReceiptSessionID: dispatchSessionID,
		FiscalSaleID: "fiscal-sale", ExecutionAuthorizationID: "auth-disp-1",
		State: "OPEN", OperatorCode: "A001",
	}
	data, _ := json.Marshal(map[string]any{
		"orders": map[string]domain.Order{dispatchOrderID: order},
		"shifts": map[string]domain.Shift{"disp-shift": {ID: "disp-shift", TenantID: "tenant-a", EmployeeID: "disp-emp"}},
		"employees": map[string]domain.Employee{
			"disp-emp": {ID: "disp-emp", TenantID: "tenant-a", Active: true, Status: "ACTIVE"},
		},
	})
	svc, err := domain.NewPersistentService("https://unused", "2026-08-07", &apiStore{b: data})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.BindEmployeeIdentity("tenant-a", "disp-emp", "disp-subj", "disp-iss"); err != nil {
		t.Fatal(err)
	}
	tok, err := auth.Sign(auth.Claims{Subject: "disp-subj", Issuer: "disp-iss", TenantID: "tenant-a", Roles: []string{"CASHIER"}, Scope: "fiscal.base", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte(testAuthHMACKey))
	if err != nil {
		t.Fatal(err)
	}
	dig := sha256.Sum256([]byte(tok))
	if _, err = svc.RegisterOperatorSession("tenant-a", "disp-emp", "00000000-0000-4000-8000-000000000041", base64.RawURLEncoding.EncodeToString(dig[:]), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	h := &handler{s: svc, execution: br}
	mux := http.NewServeMux()
	mux.HandleFunc(dispatchPath, h.dispatchExecutionAuthorization)
	return auth.Middleware(testAuthHMACKey, enforceSuccessResponses(mux)), tok
}

func dispatchReq(t *testing.T, h http.Handler, tok string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"order_id":"` + dispatchOrderID + `"}`
	r := httptest.NewRequest(http.MethodPost, dispatchPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+tok)
	r.Header.Set("X-App-Instance-Id", "00000000-0000-4000-8000-000000000041")
	r.Header.Set("X-Api-Version", "2026-08-07")
	r.Header.Set("Idempotency-Key", "dispatch-test-idempotency-key-001")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestDispatchTimeoutReturnsUnknownNot409(t *testing.T) {
	h, tok := makeDispatchHandler(t, &dispatchBridge{err: fiscalintegration.ErrDispatchOutcomeUnknown})
	w := dispatchReq(t, h, tok)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["state"] != "UNKNOWN" {
		t.Fatalf("expected state UNKNOWN, got %v", out["state"])
	}
}

func TestDispatch5xxMapsTo503(t *testing.T) {
	h, tok := makeDispatchHandler(t, &dispatchBridge{err: &fiscalintegration.HTTPStatusError{StatusCode: 500, Body: "error"}})
	w := dispatchReq(t, h, tok)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDispatch403MapsTo403(t *testing.T) {
	h, tok := makeDispatchHandler(t, &dispatchBridge{err: &fiscalintegration.HTTPStatusError{StatusCode: 403, Body: "denied"}})
	w := dispatchReq(t, h, tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
