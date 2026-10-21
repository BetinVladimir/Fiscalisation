package api

import (
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"net/http"
)

func (h *handler) mutateNativeOrderLine(w http.ResponseWriter, r *http.Request, orderID, lineID string) {
	expected, ok := parseIfMatch(r)
	if !ok {
		problem(w, http.StatusPreconditionRequired, "If-Match required")
		return
	}
	var replacement *domain.Line
	if r.Method == http.MethodPatch {
		var input domain.Line
		if !decode(w, r, &input) {
			return
		}
		replacement = &input
		if input.Discount != nil && input.Discount.Amount != "0.00" {
			if claims, authenticated := auth.ClaimsFrom(r.Context()); authenticated && !claimHasRole(claims, "SUPERVISOR") && !claimHasRole(claims, "ADMIN") {
				problem(w, http.StatusForbidden, "manager role required for discount")
				return
			}
		}
	}
	result, err := h.s.MutateOrderLineForTenant(orderID, lineID, expected, replacement, tenantID(r))
	if err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	write(w, http.StatusOK, result)
}
