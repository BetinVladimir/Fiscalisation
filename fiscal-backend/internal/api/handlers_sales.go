package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"fiscalisation/fiscal-backend/internal/domain"
)

func (h *Handler) sales(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if mode := strings.TrimSpace(r.URL.Query().Get("mode")); mode != "" {
			problem(w, 400, "SALE_MODE_NOT_SUPPORTED")
			return
		}
		write(w, 200, map[string]any{"items": h.svc.SalesForTenant(tenantID(r), r.URL.Query().Get("operator_id"), r.URL.Query().Get("register_id"), r.URL.Query().Get("state"))})
		return
	}
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	var in domain.CreateSale
	if json.Unmarshal(body, &in) != nil {
		problem(w, 400, "INVALID_JSON")
		return
	}
	in.TenantID = tenantID(r)
	v, e := h.svc.CreateSale(in)
	if e != nil {
		problem(w, 422, "INVALID_SALE")
		return
	}
	h.saveReplay(w, r, body, 201, v)
}

func (h *Handler) openSaleWithLine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	var in domain.OpenSaleWithFirstLineRequest
	if json.Unmarshal(body, &in) != nil {
		problem(w, 400, "INVALID_JSON")
		return
	}
	in.TenantID = tenantID(r)
	v, err := h.svc.OpenSaleWithFirstLine(in)
	if err != nil {
		log.Printf("sale open rejected tenant=%s workstation=%s error=%v", in.TenantID, in.WorkstationID, err)
		problem(w, 409, "SALE_OPEN_REJECTED")
		return
	}
	w.Header().Set("ETag", strconv.FormatInt(v.Version, 10))
	h.saveReplay(w, r, body, 201, v)
}

func (h *Handler) sale(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/sales/"), "/")
	parts := strings.Split(p, "/")
	id := parts[0]
	colonAction := ""
	if strings.HasSuffix(id, ":cancel") {
		id, colonAction = strings.TrimSuffix(id, ":cancel"), "cancel"
	}
	if strings.HasSuffix(id, ":reverse") {
		id, colonAction = strings.TrimSuffix(id, ":reverse"), "reversals"
	}
	if strings.HasSuffix(id, ":finalize") {
		id, colonAction = strings.TrimSuffix(id, ":finalize"), "finalize"
	}
	if !h.authorizeSale(w, r, id) {
		return
	}
	if len(parts) == 3 && parts[1] == "lines" && (r.Method == http.MethodPatch || (r.Method == http.MethodPost && strings.HasSuffix(parts[2], ":cancel"))) {
		lineID := strings.TrimSuffix(parts[2], ":cancel")
		expected, ok := ifMatch(r)
		if !ok {
			problem(w, 428, "IF_MATCH_REQUIRED")
			return
		}
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if h.replay(w, r, body) {
			return
		}
		var v domain.Sale
		var e error
		if r.Method == http.MethodPatch {
			var in domain.SaleLine
			if json.Unmarshal(body, &in) != nil {
				problem(w, 400, "INVALID_JSON")
				return
			}
			v, e = h.svc.ChangeLineExpectedForTenant(id, lineID, expected, in, tenantID(r))
		} else {
			v, e = h.svc.CancelLineExpectedForTenant(id, lineID, expected, tenantID(r))
		}
		if e != nil {
			problem(w, 409, "SALE_LINE_REJECTED")
			return
		}
		w.Header().Set("ETag", strconv.FormatInt(v.Version, 10))
		h.saveReplay(w, r, body, 200, v)
		return
	}
	if colonAction != "" {
		if r.Method != http.MethodPost {
			problem(w, 405, "METHOD_NOT_ALLOWED")
			return
		}
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if h.replay(w, r, body) {
			return
		}
		if colonAction == "cancel" {
			v, e := h.svc.CancelSaleForTenant(id, tenantID(r))
			if e != nil {
				problem(w, 409, "SALE_CANCEL_REJECTED")
				return
			}
			h.saveReplay(w, r, body, 202, v)
			return
		}
		if colonAction == "finalize" {
			var in domain.SaleFinalizeRequest
			if json.Unmarshal(body, &in) != nil {
				problem(w, 400, "INVALID_JSON")
				return
			}
			v, e := h.svc.FinalizeSaleForTenant(id, in, tenantID(r))
			if e != nil {
				log.Printf("sale finalize rejected tenant=%s sale=%s: %v", tenantID(r), id, e)
				problem(w, 409, "SALE_FINALIZE_REJECTED")
				return
			}
			h.saveReplay(w, r, body, 202, v)
			return
		}
		var in struct {
			ReasonCode              string `json:"reason_code"`
			OriginalFiscalReference string `json:"original_fiscal_reference"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.ReverseForTenantWithReference(id, in.ReasonCode, in.OriginalFiscalReference, tenantID(r))
		if e != nil {
			problem(w, 409, "REVERSAL_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 202, v)
		return
	}
	if len(parts) == 1 && r.Method == "GET" {
		v, e := h.svc.GetSaleForTenant(id, tenantID(r))
		if e != nil {
			problem(w, 404, "SALE_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if len(parts) == 2 && parts[1] == "receipt" && r.Method == "GET" {
		v, e := h.svc.ReceiptForTenant(id, tenantID(r))
		if e != nil {
			problem(w, 409, "RECEIPT_UNAVAILABLE")
			return
		}
		write(w, 200, v)
		return
	}
	if len(parts) != 2 || r.Method != "POST" {
		problem(w, 404, "NOT_FOUND")
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	switch parts[1] {
	case "lines":
		expected, ok := ifMatch(r)
		if !ok {
			problem(w, 428, "IF_MATCH_REQUIRED")
			return
		}
		var in domain.SaleLine
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.AddLineExpectedForTenant(id, expected, in, tenantID(r))
		if e != nil {
			problem(w, 409, "SALE_LINE_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 200, v)
	case "payments", "payment-intents":
		if parts[1] == "payment-intents" {
			expected, ok := ifMatch(r)
			if !ok {
				problem(w, 428, "IF_MATCH_REQUIRED")
				return
			}
			sale, e := h.svc.GetSaleForTenant(id, tenantID(r))
			if e != nil || sale.Version != expected {
				problem(w, 409, "SALE_VERSION_CONFLICT")
				return
			}
		}
		var in domain.PaymentRequest
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.PayForTenant(id, in, tenantID(r))
		if e != nil {
			log.Printf("payment rejected tenant=%s sale=%s: %v", tenantID(r), id, e)
			problem(w, 409, "PAYMENT_REJECTED")
			return
		}
		// PAYMENT_ACCEPTED is an internal aggregate state: the tender was
		// accepted, while the sale still has an outstanding balance. Expose
		// it as the canonical in-progress FiscalOperation state.
		if v.State == "PAYMENT_ACCEPTED" {
			v.State = "EXECUTING"
		}
		h.saveReplay(w, r, body, 202, v)
	case "cancel":
		v, e := h.svc.CancelSaleForTenant(id, tenantID(r))
		if e != nil {
			problem(w, 409, "SALE_CANCEL_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 202, v)
	case "reversals":
		var in struct {
			ReasonCode              string `json:"reason_code"`
			OriginalFiscalReference string `json:"original_fiscal_reference"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.ReverseForTenantWithReference(id, in.ReasonCode, in.OriginalFiscalReference, tenantID(r))
		if e != nil {
			problem(w, 409, "REVERSAL_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 202, v)
	default:
		problem(w, 404, "NOT_FOUND")
	}
}

func (h *Handler) operations(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	items := h.svc.Operations(tenantID(r))
	filtered := items[:0]
	for _, item := range items {
		if state := r.URL.Query().Get("state"); state != "" && item.State != state {
			continue
		}
		if register := r.URL.Query().Get("register_id"); register != "" {
			sale, err := h.svc.GetSaleForTenant(item.SaleID, tenantID(r))
			if err != nil || sale.RegisterID != register {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	page, err := paginate(r, filtered)
	if err != nil {
		problem(w, 400, "INVALID_PAGINATION")
		return
	}
	write(w, 200, page)
}

func (h *Handler) operation(w http.ResponseWriter, r *http.Request) {
	p := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/operations/"), "/"), "/")
	id := p[0]
	colonReconcile := strings.HasSuffix(id, ":reconcile")
	if colonReconcile {
		id = strings.TrimSuffix(id, ":reconcile")
	}
	v, e := h.svc.GetOperationForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "OPERATION_NOT_FOUND")
		return
	}
	if len(p) == 1 && r.Method == "GET" {
		write(w, 200, v)
		return
	}
	if ((len(p) == 2 && p[1] == "reconcile") || colonReconcile) && r.Method == "POST" {
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if h.replay(w, r, body) {
			return
		}
		v, e = h.svc.ReconcileOperationForTenant(id, tenantID(r))
		if e != nil {
			problem(w, 409, "RECONCILIATION_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 202, v)
		return
	}
	problem(w, 405, "METHOD_NOT_ALLOWED")
}

func (h *Handler) shifts(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		items := h.svc.Shifts(tenantID(r))
		if register := strings.TrimSpace(r.URL.Query().Get("register_id")); register != "" {
			filtered := items[:0]
			for _, item := range items {
				if item.RegisterID == register {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		page, err := paginate(r, items)
		if err != nil {
			problem(w, 400, "INVALID_PAGINATION")
			return
		}
		write(w, 200, page)
		return
	}
	if r.Method != "POST" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	var in struct {
		RegisterID string `json:"register_id"`
		OperatorID string `json:"operator_id"`
	}
	if json.Unmarshal(body, &in) != nil {
		problem(w, 400, "INVALID_JSON")
		return
	}
	v, e := h.svc.OpenShift(in.RegisterID, in.OperatorID, tenantID(r))
	if e != nil {
		problem(w, 409, "SHIFT_REJECTED")
		return
	}
	h.saveReplay(w, r, body, 201, v)
}

func (h *Handler) shift(w http.ResponseWriter, r *http.Request) {
	p := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/shifts/"), "/"), "/")
	if len(p) == 1 && r.Method == "GET" {
		v, e := h.svc.GetShift(p[0], tenantID(r))
		if e != nil {
			problem(w, 404, "SHIFT_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if len(p) != 2 || p[1] != "close" || r.Method != "POST" {
		problem(w, 404, "NOT_FOUND")
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	if _, e := h.svc.GetShift(p[0], tenantID(r)); e != nil {
		problem(w, 404, "SHIFT_NOT_FOUND")
		return
	}
	v, e := h.svc.CloseShiftForTenant(p[0], tenantID(r))
	if e != nil {
		problem(w, 409, "SHIFT_CLOSE_REJECTED")
		return
	}
	operationID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	result := map[string]any{"operation_id": operationID, "type": "SHIFT_CLOSE", "state": "SYNCED", "simulated": false, "allowed_actions": []string{}, "created_at": v.ClosedAt, "updated_at": v.UpdatedAt}
	h.saveReplay(w, r, body, 202, result)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	p := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/registers/"), "/"), "/")
	if len(p) == 1 && p[0] != "" {
		if r.Method == "GET" {
			v, e := h.svc.GetResource("register", p[0], tenantID(r))
			if e != nil {
				problem(w, 404, "REGISTER_NOT_FOUND")
				return
			}
			write(w, 200, v)
			return
		}
		if r.Method == "PATCH" {
			h.resourceItem("register", "/public/v1/registers/").ServeHTTP(w, r)
			return
		}
	}
	if len(p) == 2 && p[1] == "composite-bindings" && r.Method == "GET" {
		v, e := h.svc.CompositeBindings(p[0], tenantID(r))
		if e != nil {
			problem(w, 404, "REGISTER_NOT_FOUND")
			return
		}
		write(w, 200, map[string]any{"items": v})
		return
	}
	if len(p) < 2 || r.Method != "POST" {
		problem(w, 404, "NOT_FOUND")
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	switch p[1] {
	case "composite-bindings":
		var in struct {
			Profile                 string `json:"profile"`
			AdapterDeviceID         string `json:"adapter_device_id"`
			FiscalDeviceID          string `json:"fiscal_device_id"`
			PaymentDeviceID         string `json:"payment_device_id"`
			ExpectedRegisterVersion int64  `json:"expected_register_version"`
			BindingID               string `json:"binding_id"`
			Generation              int64  `json:"generation"`
			ExpectedVersion         int64  `json:"expected_version"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		if len(p) == 3 && p[2] == "apply" {
			if actorSubject(r) == "" || actorSubject(r) != in.AdapterDeviceID {
				problem(w, 403, "ADAPTER_IDENTITY_REQUIRED")
				return
			}
			v, e := h.svc.ApplyCompositeBinding(p[0], in.BindingID, tenantID(r), in.AdapterDeviceID, in.Generation)
			if e != nil {
				problem(w, 409, "COMPOSITE_BINDING_APPLY_REJECTED")
				return
			}
			h.saveReplay(w, r, body, 200, v)
			return
		}
		if len(p) == 3 && p[2] == "disable" {
			v, e := h.svc.DisableCompositeBinding(p[0], in.BindingID, tenantID(r), in.ExpectedVersion)
			if e != nil {
				problem(w, 409, "COMPOSITE_BINDING_DISABLE_REJECTED")
				return
			}
			h.saveReplay(w, r, body, 200, v)
			return
		}
		v, e := h.svc.CreateCompositeBinding(p[0], tenantID(r), in.Profile, in.AdapterDeviceID, in.FiscalDeviceID, in.PaymentDeviceID, in.ExpectedRegisterVersion)
		if e != nil {
			problem(w, 409, "COMPOSITE_BINDING_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 202, v)
	case "bindings":
		var in struct {
			DeviceID   string `json:"device_id"`
			Role       string `json:"role"`
			ActiveFrom string `json:"active_from"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.BindRegister(p[0], tenantID(r), in.DeviceID, in.Role, in.ActiveFrom)
		if e != nil {
			problem(w, 409, "BINDING_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 201, v)
	case "connectivity-probes":
		v, e := h.svc.Connectivity(p[0], tenantID(r))
		if e != nil {
			problem(w, 500, "PROBE_PERSIST_FAILED")
			return
		}
		h.saveReplay(w, r, body, 202, v)
	case "ble-sessions":
		var in struct {
			OperatorID    string `json:"operator_id"`
			AppInstanceID string `json:"app_instance_id"`
			PublicKey     string `json:"public_key"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.BLESession(p[0], in.OperatorID, in.AppInstanceID, tenantID(r), actorSubject(r), in.PublicKey)
		if e != nil {
			problem(w, 503, "BLE_SESSION_UNAVAILABLE")
			return
		}
		h.saveReplay(w, r, body, 201, v)
	case "cash-movements":
		var in struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.FiscalOperation(p[0], in.Type, tenantID(r))
		if e != nil {
			problem(w, 422, "INVALID_CASH_MOVEMENT")
			return
		}
		h.saveReplay(w, r, body, 202, v)
	case "reports":
		var in struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.CreateReport(p[0], in.Type, tenantID(r))
		if e != nil {
			problem(w, 422, "INVALID_REPORT")
			return
		}
		h.saveReplay(w, r, body, 202, v)
	default:
		problem(w, 404, "NOT_FOUND")
	}
}
