package api

import (
	"errors"
	"fiscalisation/beeminipos-backend/internal/domain"
	"net/http"
)

// Read one authoritative snapshot, then authorize its actual shift and fence
// every intervening mutation before issuing the read-only capability.
// This resource grants reads only; it never prepares, dispatches or retries money.
func (h *handler) orderRecovery(w http.ResponseWriter, r *http.Request, id string) {
	order, epoch, err := h.s.RecoveryOrderForTenant(id, tenantID(r))
	if err != nil {
		if errors.Is(err, domain.ErrRecoveryOrderNotFound) {
			problem(w, http.StatusNotFound, "not found")
		} else {
			problem(w, http.StatusServiceUnavailable, "RECOVERY_STATE_UNAVAILABLE")
		}
		return
	}
	if !h.authorizeShiftActor(w, r, order.ShiftID) {
		return
	}
	if !h.s.RecoveryReadCurrent(epoch) {
		problem(w, http.StatusServiceUnavailable, "RECOVERY_STATE_UNAVAILABLE")
		return
	}
	actions := []string{}
	associated := order.FiscalRegisterID != "" && order.CheckoutOperationID != "" && order.ReceiptSessionID != "" && (order.FiscalOperationID == "" || order.FiscalOperationID == order.CheckoutOperationID)
	switch order.State {
	case "FISCAL_PENDING", "UNKNOWN", "COMPLETED", "REVERSED", "FAILED":
		if associated {
			actions = append(actions, "READ")
		}
	}
	write(w, http.StatusOK, map[string]any{"order_id": order.ID, "shift_id": order.ShiftID, "register_id": order.RegisterID, "operator_code": order.OperatorCode, "external_id": order.ExternalID, "order_version": order.Version, "state": order.State, "allowed_actions": actions, "client_operation_id": order.CheckoutOperationID, "receipt_session_id": order.ReceiptSessionID, "fiscal_sale_id": order.FiscalSaleID, "fiscal_register_id": order.FiscalRegisterID})
}
