package integration

import (
	"errors"
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
)

const ExecutionAuthorizationIssueScope = "execution_authorizations:issue"

type ExecutionAuthorizationIssuer interface {
	DispatchDelegatedExecution(domain.DelegatedExecutionPrincipal, string) (domain.Operation, error)
	IssueDelegatedExecutionAuthorization(domain.DelegatedExecutionPrincipal, domain.DelegatedExecutionRequest, string) (domain.DelegatedExecutionResponse, error)
}

func (h *HTTPHandler) issueExecutionAuthorization(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Scopes[ExecutionAuthorizationIssueScope] {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "EXECUTION_AUTHORIZATION_CAPABILITY_REQUIRED"}})
		return
	}
	if h.ExecutionAuthorizations == nil {
		writeError(w, errors.New("execution authorization issuer unavailable"))
		return
	}
	var in domain.DelegatedExecutionRequest
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.ExecutionAuthorizations.IssueDelegatedExecutionAuthorization(domain.DelegatedExecutionPrincipal{TenantID: p.TenantID, SystemID: p.SystemID, CredentialID: p.CredentialID, CanIssue: true}, in, r.Header.Get("Idempotency-Key"))
	if err != nil {
		if errors.Is(err, domain.ErrExecutionDelegationDenied) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "EXECUTION_DELEGATION_DENIED"}})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *HTTPHandler) dispatchExecutionAuthorization(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Scopes[ExecutionAuthorizationIssueScope] {
		writeJSON(w, 403, map[string]any{"error": map[string]any{"code": "EXECUTION_AUTHORIZATION_CAPABILITY_REQUIRED"}})
		return
	}
	if h.ExecutionAuthorizations == nil {
		writeError(w, errors.New("execution issuer unavailable"))
		return
	}
	var in struct {
		AuthorizationID string `json:"authorization_id"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.ExecutionAuthorizations.DispatchDelegatedExecution(domain.DelegatedExecutionPrincipal{TenantID: p.TenantID, SystemID: p.SystemID, CredentialID: p.CredentialID, CanIssue: true}, in.AuthorizationID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 202, out)
}
