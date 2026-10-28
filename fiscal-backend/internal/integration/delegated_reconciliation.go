package integration

import (
	"errors"
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
)

const PaymentReconciliationAuthorizeScope = "payment_reconciliations:authorize"

type PaymentReconciliationDelegator interface {
	IssueDelegatedPaymentReconciliation(domain.DelegatedReconciliationPrincipal, domain.DelegatedReconciliationRequest, string) (domain.PaymentReconciliationCommand, error)
}

func (h *HTTPHandler) authorizePaymentReconciliation(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Scopes[PaymentReconciliationAuthorizeScope] {
		writeJSON(w, 403, map[string]any{"error": map[string]any{"code": "PAYMENT_RECONCILIATION_CAPABILITY_REQUIRED"}})
		return
	}
	issuer, ok := h.ExecutionAuthorizations.(PaymentReconciliationDelegator)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": map[string]any{"code": "PAYMENT_RECONCILIATION_UNAVAILABLE"}})
		return
	}
	var in domain.DelegatedReconciliationRequest
	if e := decode(r, &in); e != nil {
		writeError(w, e)
		return
	}
	out, e := issuer.IssueDelegatedPaymentReconciliation(domain.DelegatedReconciliationPrincipal{TenantID: p.TenantID, SystemID: p.SystemID, CredentialID: p.CredentialID, CanAuthorize: true}, in, r.Header.Get("Idempotency-Key"))
	if errors.Is(e, domain.ErrReconciliationDelegationDenied) {
		writeJSON(w, 403, map[string]any{"error": map[string]any{"code": "RECONCILIATION_DELEGATION_DENIED"}})
		return
	}
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, 201, out)
}
