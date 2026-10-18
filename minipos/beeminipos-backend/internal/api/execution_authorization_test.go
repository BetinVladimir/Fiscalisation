package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"net/http"
	"testing"
	"time"
)

type executionBridgeSpy struct {
	calls   int
	company string
	request fiscalintegration.ExecutionAuthorizationRequest
}

func (s *executionBridgeSpy) ExecutionIdentity(context.Context, string, string, string) (fiscalintegration.ExecutionIdentityContext, error) {
	return fiscalintegration.ExecutionIdentityContext{TenantID: "fiscal-tenant", ActorSubject: "namespaced-actor"}, nil
}
func (s *executionBridgeSpy) IssueExecutionAuthorization(_ context.Context, company string, in fiscalintegration.ExecutionAuthorizationRequest, _ string) (fiscalintegration.ExecutionAuthorizationResponse, error) {
	s.calls++
	s.company = company
	s.request = in
	session, _ := json.Marshal(map[string]any{"session_id": "00000000-0000-4000-8000-000000000050", "tenant_id": "fiscal-tenant", "workstation_id": "fiscal-register", "operator_id": "fiscal-operator", "operator_code": "A001", "app_instance_id": in.AppInstanceID, "expires_at": time.Now().Add(time.Minute), "created_at": time.Now()})
	return fiscalintegration.ExecutionAuthorizationResponse{Authorization: fiscalintegration.ExecutionAuthorizationEnvelope{Algorithm: "ES256_DER", KeyID: "root", Payload: func() string {
		raw, _ := json.Marshal(map[string]any{"tenant_id": "fiscal-tenant", "location_id": "fiscal-location", "register_id": "fiscal-register", "operator_id": "fiscal-operator", "edge_device_id": "fiscal-edge", "binding_generation": 1, "nbf": time.Now().Add(-time.Second).Unix(), "exp": time.Now().Add(5 * time.Minute).Unix()})
		return base64.RawURLEncoding.EncodeToString(raw)
	}(), SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Signature: "signature"}, WorkstationSession: session}, nil
}
func TestExecutionBridgeUsesPersistedCallerSessionEmployeeAndShift(t *testing.T) {
	const tenant, issuer, secret = "source-tenant", "https://identity.example", "test-execution-secret"
	s := domain.NewService("http://unused", "2026-08-07")
	employee, err := s.CreateEmployee(domain.Employee{TenantID: tenant, FirstName: "Ada", LastName: "Cashier", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindEmployeeIdentity(tenant, employee.ID, "subject", issuer); err != nil {
		t.Fatal(err)
	}
	const register = "00000000-0000-4000-8000-000000000011"
	if _, err = s.SaveConfiguration(tenant, 0, domain.Configuration{LocationName: "Shop", WorkstationName: "POS", LocationID: "00000000-0000-4000-8000-000000000012", FiscalRegisterID: register, FiscalAdapterID: "00000000-0000-4000-8000-000000000013", BindingGeneration: 1, AdapterBaseURL: "http://adapter/beeloy/local/v1"}); err != nil {
		t.Fatal(err)
	}
	shift, err := s.OpenShiftForTenant(register, employee.ID, tenant)
	if err != nil {
		t.Fatal(err)
	}
	token := operatorToken(t, secret, "subject", issuer, tenant, "CASHIER")
	digest := sha256.Sum256([]byte(token))
	hash := base64.RawURLEncoding.EncodeToString(digest[:])
	const app = "00000000-0000-4000-8000-000000000001"
	if _, err = s.RegisterOperatorSession(tenant, employee.ID, app, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	spy := &executionBridgeSpy{}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalECPrivateKey(key)
	h := &handler{s: s, execution: spy, c: config.Config{LocalTokenSigningKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})), LocalTokenIssuer: "https://minipos.example", LocalTokenSigningKID: "source-local-token"}}
	mux := http.NewServeMux()
	mux.HandleFunc("/public/v1/minipos/execution-authorizations", h.executionAuthorization)
	route := auth.MiddlewareWithRevocation(secret, func(c auth.Claims) bool { return s.OperatorSessionRevoked(c.TenantID, c.TokenHash) }, apiVersion("2026-08-07", enforceSuccessResponses(mux)))
	body := `{"shift_id":"` + shift.ID + `","client_signing_public_key_der_base64":"browser-public-key"}`
	out := operatorRequest(route, http.MethodPost, "/public/v1/minipos/execution-authorizations", token, "00000000-0000-4000-8000-000000000099", body)
	if out.Code != 201 || spy.calls != 1 || spy.company != tenant || spy.request.SourceOperatorID != employee.ID || spy.request.SourceRegisterID != register || spy.request.IdentityIssuer != issuer || spy.request.IdentitySubject != "subject" || spy.request.SourceSessionID != hash || spy.request.AppInstanceID != app || spy.request.ServerSaleID != "" {
		t.Fatalf("trusted context missing: %d %s %+v", out.Code, out.Body.String(), spy.request)
	}
	ctx := operatorRequest(route, http.MethodGet, "/public/v1/minipos/execution-authorizations?shift_id="+shift.ID, token, "", "")
	if ctx.Code != 200 {
		t.Fatal(ctx.Code, ctx.Body.String())
	}
	for _, bad := range []string{`{"shift_id":"` + shift.ID + `","client_signing_public_key_der_base64":"key","operator_id":"forged"}`, `{"shift_id":"00000000-0000-4000-8000-000000000099","client_signing_public_key_der_base64":"key"}`, `{"shift_id":"` + shift.ID + `","client_signing_public_key_der_base64":"key"} {}`} {
		out := operatorRequest(route, http.MethodPost, "/public/v1/minipos/execution-authorizations", token, "00000000-0000-4000-8000-000000000098", bad)
		if out.Code < 400 || spy.calls != 1 {
			t.Fatal("untrusted body reached root", out.Code)
		}
	}
	if _, err = s.RevokeOperatorSession(tenant, hash); err != nil {
		t.Fatal(err)
	}
	denied := operatorRequest(route, http.MethodPost, "/public/v1/minipos/execution-authorizations", token, "00000000-0000-4000-8000-000000000097", body)
	if denied.Code != 401 || spy.calls != 1 {
		t.Fatal("revoked session delegated", denied.Code)
	}
}

func (s *executionBridgeSpy) DispatchExecutionAuthorization(context.Context, string, string) (fiscalintegration.FiscalOperation, error) {
	return fiscalintegration.FiscalOperation{}, nil
}
