package api

import (
	"fiscalisation/beeminipos-backend/internal/domain"
	"net/http"
)

func (h *handler) orderOperation(w http.ResponseWriter, r *http.Request, orderID, operationID string) {
	// order() has already applied tenant and shift-actor authorization.
	order, err := h.s.OrderForTenant(orderID, tenantID(r))
	if err != nil {
		problem(w, http.StatusNotFound, "not found")
		return
	}
	if operationID == "" || order.CheckoutOperationID != operationID || order.ReceiptSessionID == "" || order.RegisterID == "" {
		problem(w, http.StatusConflict, "FISCAL_OPERATION_ASSOCIATION_UNAVAILABLE")
		return
	}
	if h.operations == nil {
		problem(w, http.StatusServiceUnavailable, "FISCAL_OPERATION_LOOKUP_UNAVAILABLE")
		return
	}
	operation, err := h.operations.GetFiscalOperation(r.Context(), tenantID(r), operationID)
	if err != nil {
		problem(w, http.StatusBadGateway, "FISCAL_OPERATION_LOOKUP_UNCERTAIN")
		return
	}
	if operation.TenantID == "" || operation.SaleID == "" || operation.OperationID != operationID || operation.ClientOperationID != operationID || operation.ReceiptSessionID != order.ReceiptSessionID || (order.FiscalSaleID != "" && operation.SaleID != order.FiscalSaleID) || operation.RegisterID != domain.FiscalRegisterForOrder(order) || (operation.Type != "SALE_FINALIZE" && operation.Type != "FISCAL_SALE") || operation.Version < 1 || operation.CreatedAt.IsZero() || operation.UpdatedAt.IsZero() || (operation.State == "FISCALIZED" && operation.FiscalReference == "") {
		problem(w, http.StatusBadGateway, "FISCAL_OPERATION_ASSOCIATION_MISMATCH")
		return
	}
	if order.FiscalSaleID == "" {
		sale, readErr := h.operations.GetFiscalSale(r.Context(), tenantID(r), operation.SaleID)
		if readErr != nil || sale.SaleID == "" || sale.SaleID != operation.SaleID || sale.TenantID != operation.TenantID || order.ExternalID == "" || sale.ExternalID != order.ExternalID || sale.RegisterID != order.RegisterID || order.OperatorCode == "" || sale.OperatorID != order.OperatorCode {
			problem(w, http.StatusBadGateway, "FISCAL_SALE_ASSOCIATION_MISMATCH")
			return
		}
	}
	projected, projectionErr := h.s.ApplyValidatedCheckoutLookup(tenantID(r), order, domain.CheckoutLookup{OperationID: operation.OperationID, ReceiptSessionID: operation.ReceiptSessionID, SaleID: operation.SaleID, RegisterID: operation.RegisterID, State: operation.State, FiscalReference: operation.FiscalReference, Version: operation.Version})
	if projectionErr != nil {
		problem(w, http.StatusConflict, "FISCAL_OPERATION_PROJECTION_CONFLICT")
		return
	}
	// Preserve previously validated successful proof if a delayed GET is UNKNOWN.
	if (projected.State == "COMPLETED" || projected.State == "REVERSED") && projected.ReceiptReference != "" {
		operation.State = "FISCALIZED"
		operation.FiscalReference = projected.ReceiptReference
		if projected.FiscalVersion > operation.Version {
			operation.Version = projected.FiscalVersion
		}
	}
	if operation.AllowedActions == nil {
		operation.AllowedActions = []string{}
	}
	write(w, http.StatusOK, operation)
}
