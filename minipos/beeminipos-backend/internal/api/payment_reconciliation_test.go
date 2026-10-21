package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type reconciliationBridgeSpy struct {
	executionBridgeSpy
	decisions int
	company   string
	request   fiscalintegration.PaymentReconciliationDelegation
}

func (s *reconciliationBridgeSpy) AuthorizePaymentReconciliation(_ context.Context, company string, in fiscalintegration.PaymentReconciliationDelegation, key string) (fiscalintegration.PaymentReconciliationCommand, error) {
	s.decisions++
	s.company = company
	s.request = in
	return fiscalintegration.PaymentReconciliationCommand{Command: "PAYMENT_RECONCILE", ClientOperationID: "10000000-0000-4000-8000-000000000001", Authorization: fiscalintegration.ExecutionAuthorizationEnvelope{Algorithm: "ES256_DER", KeyID: "root", Payload: "e30", SHA256: strings.Repeat("a", 64), Signature: "signed"}}, nil
}
func TestReconciliationSourceRequiresActualAndPersistedAdminSession(t *testing.T) {
	const tenant, issuer, secret, app = "tenant", "https://identity.test", "test-secret", "00000000-0000-4000-8000-000000000001"
	for _, tc := range []struct {
		jwtRole, employeeRole string
		active                bool
		want                  int
	}{{"CASHIER", "ADMIN", true, 403}, {"ADMIN", "CASHIER", true, 403}, {"ADMIN", "ADMIN", false, 401}, {"ADMIN", "ADMIN", true, 201}} {
		t.Run(tc.jwtRole+tc.employeeRole+time.Duration(tc.want).String(), func(t *testing.T) {
			s := domain.NewService("unused", "2026-08-07")
			employee, e := s.CreateEmployee(domain.Employee{TenantID: tenant, FirstName: "Admin", LastName: "Operator", OperatorCode: "A001", Roles: []string{tc.employeeRole}})
			if e != nil {
				t.Fatal(e)
			}
			s.BindEmployeeIdentity(tenant, employee.ID, "admin-subject", issuer)
			const register = "50000000-0000-4000-8000-000000000001"
			if _, e := s.SaveConfiguration(tenant, 0, domain.Configuration{LocationName: "Shop", WorkstationName: "POS", LocationID: "60000000-0000-4000-8000-000000000001", FiscalAdapterID: "70000000-0000-4000-8000-000000000001", FiscalRegisterID: register, BindingGeneration: 1, AdapterBaseURL: "http://adapter/beeloy/local/v1"}); e != nil {
				t.Fatal(e)
			}
			shift, e := s.OpenShiftForTenant(register, employee.ID, tenant)
			if e != nil {
				t.Fatal(e)
			}
			token := operatorToken(t, secret, "admin-subject", issuer, tenant, tc.jwtRole)
			digest := sha256.Sum256([]byte(token))
			hash := base64.RawURLEncoding.EncodeToString(digest[:])
			if tc.active {
				s.RegisterOperatorSession(tenant, employee.ID, app, hash, time.Now().Add(time.Hour))
			}
			spy := &reconciliationBridgeSpy{}
			h := &handler{s: s, execution: spy}
			mux := http.NewServeMux()
			mux.HandleFunc("/public/v1/minipos/payment-reconciliations:authorize", h.authorizePaymentReconciliation)
			route := auth.Middleware(secret, mux)
			body := `{"shift_id":"` + shift.ID + `","decision":{"operation_id":"10000000-0000-4000-8000-000000000002","payment_id":"10000000-0000-4000-8000-000000000003","observed_state":"CARD_DECLINED","evidence_reference":"bank-case","evidence_sha256":"` + strings.Repeat("b", 64) + `"}}`
			call := func(body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("POST", "/public/v1/minipos/payment-reconciliations:authorize", strings.NewReader(body))
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("X-App-Instance-Id", app)
				r.Header.Set("Idempotency-Key", "decision-key-0001")
				w := httptest.NewRecorder()
				route.ServeHTTP(w, r)
				return w
			}
			w := call(body)
			if w.Code != tc.want {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.want == 201 {
				if spy.decisions != 1 || spy.company != tenant || spy.request.SourceOperatorID != employee.ID || spy.request.IdentityIssuer != issuer || spy.request.IdentitySubject != "admin-subject" || spy.request.SourceSessionID != hash || spy.request.SourceRegisterID != register {
					t.Fatal("untrusted context", spy.request)
				}
				forged := strings.TrimSuffix(body, "}") + `,"actor_subject":"forged"}`
				if w := call(forged); w.Code != 400 || spy.decisions != 1 {
					t.Fatal("caller actor accepted", w.Code)
				}
			} else if spy.decisions != 0 {
				t.Fatal("unauthorized root call")
			}
		})
	}
}
