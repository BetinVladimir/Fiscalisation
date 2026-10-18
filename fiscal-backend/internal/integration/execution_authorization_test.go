package integration

import (
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type executionIssuerSpy struct {
	failure   error
	calls     int
	principal domain.DelegatedExecutionPrincipal
	request   domain.DelegatedExecutionRequest
}

func (s *executionIssuerSpy) IssueDelegatedExecutionAuthorization(p domain.DelegatedExecutionPrincipal, in domain.DelegatedExecutionRequest, key string) (domain.DelegatedExecutionResponse, error) {
	s.calls++
	s.principal = p
	s.request = in
	return domain.DelegatedExecutionResponse{}, s.failure
}
func TestDelegatedExecutionCapabilityBoundary(t *testing.T) {
	spy := &executionIssuerSpy{}
	h := HTTPHandler{ExecutionAuthorizations: spy}
	for _, scopes := range []map[string]bool{nil, {"operators.write": true}, {"ADMIN": true}} {
		r := httptest.NewRequest(http.MethodPost, "/integration/v1/execution-authorizations", strings.NewReader(`{}`))
		r.Header.Set("BeeFiscal-Source-Actor-Type", "ADMIN")
		w := httptest.NewRecorder()
		h.issueExecutionAuthorization(w, r, Principal{TenantID: "tenant", SystemID: "source", CredentialID: "credential", Scopes: scopes})
		if w.Code != 403 || spy.calls != 0 {
			t.Fatal("audit or unrelated capability authorized", w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/integration/v1/execution-authorizations", strings.NewReader(`{"source_operator_id":"employee"}`))
	r.Header.Set("Idempotency-Key", "request")
	w := httptest.NewRecorder()
	h.issueExecutionAuthorization(w, r, Principal{TenantID: "tenant", SystemID: "source", CredentialID: "credential", Scopes: map[string]bool{ExecutionAuthorizationIssueScope: true}})
	if w.Code != 201 || spy.calls != 1 || spy.principal.TenantID != "tenant" || spy.principal.SystemID != "source" || !spy.principal.CanIssue {
		t.Fatal("authenticated principal lost", w.Code, spy.principal)
	}
	for _, body := range []string{`{"tenant_id":"foreign"}`, `{"source_operator_id":"employee"} {}`, `{"actor_subject":"forged"}`} {
		r := httptest.NewRequest(http.MethodPost, "/integration/v1/execution-authorizations", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.issueExecutionAuthorization(w, r, Principal{Scopes: map[string]bool{ExecutionAuthorizationIssueScope: true}})
		if w.Code != 400 || spy.calls != 1 {
			t.Fatal("caller authority accepted", w.Code)
		}
	}
}

func (s *executionIssuerSpy) DispatchDelegatedExecution(domain.DelegatedExecutionPrincipal, string) (domain.Operation, error) {
	return domain.Operation{}, nil
}

func TestDelegatedMappingRevocationIsAuthoritativeForbidden(t *testing.T) {
	spy := &executionIssuerSpy{failure: domain.ErrExecutionDelegationDenied}
	h := HTTPHandler{ExecutionAuthorizations: spy}
	r := httptest.NewRequest(http.MethodPost, "/integration/v1/execution-authorizations", strings.NewReader(`{"source_operator_id":"employee"}`))
	w := httptest.NewRecorder()
	h.issueExecutionAuthorization(w, r, Principal{TenantID: "tenant", SystemID: "source", CredentialID: "credential", Scopes: map[string]bool{ExecutionAuthorizationIssueScope: true}})
	if w.Code != 403 || !strings.Contains(w.Body.String(), "EXECUTION_DELEGATION_DENIED") {
		t.Fatal(w.Code, w.Body.String())
	}
}
