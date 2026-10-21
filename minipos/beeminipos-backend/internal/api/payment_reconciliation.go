package api

import (
	"context"
	"encoding/json"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"io"
	"net/http"
	"strings"
	"time"
)

type paymentReconciliationAuthorizer interface {
	AuthorizePaymentReconciliation(context.Context, string, fiscalintegration.PaymentReconciliationDelegation, string) (fiscalintegration.PaymentReconciliationCommand, error)
}

func (h *handler) authorizePaymentReconciliation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "method")
		return
	}
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok || claims.Issuer == "" || claims.TokenHash == "" {
		problem(w, 401, "authenticated source identity required")
		return
	}
	if !claimHasRole(claims, "ADMIN") {
		problem(w, 403, "ADMIN required")
		return
	}
	employee, e := h.s.EmployeeForIdentity(claims.TenantID, claims.Issuer, claims.Subject)
	if e != nil {
		problem(w, 403, "mapped administrator required")
		return
	}
	admin := false
	for _, role := range employee.Roles {
		if strings.EqualFold(role, "ADMIN") {
			admin = true
		}
	}
	if !admin {
		problem(w, 403, "persisted ADMIN employee required")
		return
	}
	app := strings.TrimSpace(r.Header.Get("X-App-Instance-Id"))
	if !h.s.OperatorSessionActive(claims.TenantID, claims.TokenHash, employee.ID, app, time.Now().UTC()) {
		problem(w, 401, "active caller-owned administrator session required")
		return
	}
	var in struct {
		ShiftID  string                                          `json:"shift_id"`
		Decision fiscalintegration.PaymentReconciliationDecision `json:"decision"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		problem(w, 400, "invalid reconciliation request")
		return
	}
	shift, e := h.s.ShiftForTenant(in.ShiftID, claims.TenantID)
	if e != nil || shift.State != "OPEN" || shift.EmployeeID != employee.ID {
		problem(w, 403, "caller-owned open administrator shift required")
		return
	}
	configuration, e := h.s.ConfigurationFor(claims.TenantID)
	if e != nil || configuration.FiscalRegisterID == "" || shift.RegisterID != configuration.FiscalRegisterID {
		problem(w, 409, "configured register and shift mismatch")
		return
	}
	issuer, ok := h.execution.(paymentReconciliationAuthorizer)
	if !ok {
		problem(w, 503, "reconciliation delegation unavailable")
		return
	}
	out, e := issuer.AuthorizePaymentReconciliation(r.Context(), claims.TenantID, fiscalintegration.PaymentReconciliationDelegation{SourceOperatorID: employee.ID, SourceRegisterID: configuration.FiscalRegisterID, IdentityIssuer: claims.Issuer, IdentitySubject: claims.Subject, SourceSessionID: claims.TokenHash, AppInstanceID: app, SourceExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(), Decision: in.Decision}, r.Header.Get("Idempotency-Key"))
	if e != nil {
		writeExecutionUpstreamError(w, e)
		return
	}
	write(w, 201, out)
}
