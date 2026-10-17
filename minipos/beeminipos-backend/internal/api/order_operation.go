package api

import (
	"net/http"
)

func (h *handler) orderOperation(w http.ResponseWriter, r *http.Request, orderID, operationID string) {
	// order() has already applied tenant and shift-actor authorization.
	order, err := h.s.OrderForTenant(orderID, tenantID(r))
	if err != nil {
		problem(w, http.StatusNotFound, "not found")
		return
	}
	if operationID == "" || order.CheckoutOperationID != operationID || order.ReceiptSessionID == "" || order.FiscalSaleID == "" || order.RegisterID == "" {
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
	if operation.OperationID != operationID || operation.ClientOperationID != operationID || operation.ReceiptSessionID != order.ReceiptSessionID || operation.SaleID != order.FiscalSaleID || operation.RegisterID != order.RegisterID || (operation.Type != "SALE_FINALIZE" && operation.Type != "FISCAL_SALE") || operation.Version < 1 || operation.CreatedAt.IsZero() || operation.UpdatedAt.IsZero() || (operation.State == "FISCALIZED" && operation.FiscalReference == "") {
		problem(w, http.StatusBadGateway, "FISCAL_OPERATION_ASSOCIATION_MISMATCH")
		return
	}
	if operation.AllowedActions == nil {
		operation.AllowedActions = []string{}
	}
	write(w, http.StatusOK, operation)
}
