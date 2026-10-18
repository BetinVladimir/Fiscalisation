package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"io"
	"net/http"
	"strings"
	"time"
)

type fiscalExecutionAuthorizer interface {
	DispatchExecutionAuthorization(context.Context, string, string) (fiscalintegration.FiscalOperation, error)
	ExecutionIdentity(context.Context, string, string, string) (fiscalintegration.ExecutionIdentityContext, error)
	IssueExecutionAuthorization(context.Context, string, fiscalintegration.ExecutionAuthorizationRequest, string) (fiscalintegration.ExecutionAuthorizationResponse, error)
}

// Browser input contains no actor/operator, fiscal tenant, register, or UNP.
func (h *handler) executionAuthorization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		problem(w, 405, "method")
		return
	}
	if h.execution == nil {
		problem(w, 503, "Fiscal execution delegation unavailable")
		return
	}
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok || claims.Issuer == "" || claims.TokenHash == "" {
		problem(w, 401, "authenticated source identity required")
		return
	}
	var in struct {
		ShiftID   string           `json:"shift_id"`
		ClientKey string           `json:"client_signing_public_key_der_base64"`
		OrderID   string           `json:"order_id,omitempty"`
		Payments  []map[string]any `json:"payments,omitempty"`
	}
	if r.Method == http.MethodGet {
		in.ShiftID = r.URL.Query().Get("shift_id")
	} else {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			problem(w, 400, "invalid execution authorization request")
			return
		}
	}
	employee, err := h.s.EmployeeForIdentity(claims.TenantID, claims.Issuer, claims.Subject)
	if err != nil {
		problem(w, 403, "mapped employee required")
		return
	}
	app := strings.TrimSpace(r.Header.Get("X-App-Instance-Id"))
	if !h.s.OperatorSessionActive(claims.TenantID, claims.TokenHash, employee.ID, app, time.Now().UTC()) {
		problem(w, 401, "active caller-owned operator session required")
		return
	}
	shift, err := h.s.ShiftForTenant(in.ShiftID, claims.TenantID)
	if err != nil || shift.State != "OPEN" || shift.EmployeeID != employee.ID {
		problem(w, 403, "caller-owned open shift required")
		return
	}
	configuration, err := h.s.ConfigurationFor(claims.TenantID)
	if err != nil || configuration.FiscalRegisterID == "" || shift.RegisterID != configuration.FiscalRegisterID {
		problem(w, 409, "configured register and shift mismatch")
		return
	}
	if r.Method == http.MethodGet {
		out, e := h.execution.ExecutionIdentity(r.Context(), claims.TenantID, claims.Issuer, claims.Subject)
		if e != nil {
			problem(w, 409, e.Error())
			return
		}
		out.SourceSessionID = claims.TokenHash
		out.AppInstanceID = app
		write(w, 200, out)
		return
	}
	request := fiscalintegration.ExecutionAuthorizationRequest{SourceOperatorID: employee.ID, SourceRegisterID: configuration.FiscalRegisterID, IdentityIssuer: claims.Issuer, IdentitySubject: claims.Subject, SourceSessionID: claims.TokenHash, AppInstanceID: app, SourceExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(), ClientSigningPublicKeyDERBase64: in.ClientKey}
	if in.OrderID != "" {
		order, e := h.s.FreezeExecutionOrder(in.OrderID, in.ShiftID, employee.ID, claims.TenantID, in.Payments)
		if e != nil {
			problem(w, 409, e.Error())
			return
		}
		plan := map[string]any{"source_order_id": order.ID, "source_shift_id": shift.ID, "items": executionReceiptLines(order.Lines), "payments": order.Payments, "expected_total": order.Total}
		request.SalePreparation, _ = json.Marshal(plan)
	} else if len(in.Payments) > 0 {
		problem(w, 400, "payments require a persisted source order")
		return
	}
	out, err := h.execution.IssueExecutionAuthorization(r.Context(), claims.TenantID, request, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeExecutionUpstreamError(w, err)
		return
	}
	if in.OrderID != "" {
		raw, e := base64.RawURLEncoding.DecodeString(out.Authorization.Payload)
		var trusted struct {
			AuthorizationID string `json:"authorization_id"`
			RegisterID      string `json:"register_id"`
			UNPAuthority    *struct {
				ServerSaleID     string `json:"server_sale_id"`
				OperationID      string `json:"operation_id"`
				ReceiptSessionID string `json:"receipt_session_id"`
			} `json:"unp_authority"`
		}
		if e != nil || json.Unmarshal(raw, &trusted) != nil || trusted.UNPAuthority == nil {
			problem(w, 502, "trusted server preparation missing")
			return
		}
		v := trusted.UNPAuthority
		if _, e = h.s.AttachPreparedFiscalExecution(in.OrderID, claims.TenantID, v.ServerSaleID, v.OperationID, v.ReceiptSessionID, trusted.RegisterID, trusted.AuthorizationID); e != nil {
			problem(w, 409, e.Error())
			return
		}
	}
	token, expires, e := h.executionTransportToken(out.Authorization, claims, shift.ID)
	if e != nil {
		problem(w, 503, e.Error())
		return
	}
	write(w, 201, executionAuthorizationResponse{ExecutionAuthorizationResponse: out, TransportToken: token, TransportTokenExpiresAt: expires})
}

func (h *handler) dispatchExecutionAuthorization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "method")
		return
	}
	if h.execution == nil {
		problem(w, 503, "Fiscal execution delegation unavailable")
		return
	}
	var in struct {
		OrderID string `json:"order_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		problem(w, 400, "invalid dispatch request")
		return
	}
	order, err := h.s.OrderForTenant(in.OrderID, tenantID(r))
	if err != nil {
		problem(w, 404, "order not found")
		return
	}
	if !h.authorizeShiftActor(w, r, order.ShiftID) {
		return
	}
	if order.ExecutionAuthorizationID == "" || order.FiscalSaleID == "" {
		problem(w, 409, "trusted fiscal preparation required")
		return
	}
	out, err := h.execution.DispatchExecutionAuthorization(r.Context(), tenantID(r), order.ExecutionAuthorizationID)
	if err != nil {
		problem(w, 409, err.Error())
		return
	}
	if out.OperationID != order.CheckoutOperationID || out.ClientOperationID != order.CheckoutOperationID || out.ReceiptSessionID != order.ReceiptSessionID || out.SaleID != order.FiscalSaleID || out.RegisterID != domain.FiscalRegisterForOrder(order) {
		problem(w, 502, "prepared operation identity mismatch")
		return
	}
	write(w, 202, out)
}

// Source catalog IDs are not fiscal product codes. Keep them in the persisted
// MiniPOS order; only the immutable fiscal line contract crosses this boundary.
func executionReceiptLines(lines []domain.Line) []map[string]any {
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		v := map[string]any{"line_id": line.LineID, "name": line.Name, "quantity": line.Quantity, "unit_price": line.UnitPrice, "tax_group": line.TaxGroup}
		if line.Discount != nil {
			v["discount"] = line.Discount
		}
		out = append(out, v)
	}
	return out
}

func writeExecutionUpstreamError(w http.ResponseWriter, err error) {
	var upstream *fiscalintegration.HTTPStatusError
	if errors.As(err, &upstream) {
		switch upstream.StatusCode {
		case 401, 403:
			problem(w, upstream.StatusCode, "Fiscal execution authority denied")
			return
		}
		if upstream.StatusCode >= 500 {
			problem(w, 503, "Fiscal issuer unavailable")
			return
		}
		problem(w, 409, err.Error())
		return
	}
	// An unavailable issuer cannot be interpreted as a negative authority decision.
	problem(w, 503, "Fiscal issuer unavailable")
}
