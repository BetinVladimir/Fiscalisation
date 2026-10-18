package api

import (
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
)

func (h *Handler) provisioningDelivery(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	if !hasAnyRole(r, "ADMIN", "SERVICE", "SERVICE_TECHNICIAN") {
		problem(w, 403, "PROVISIONING_ROLE_REQUIRED")
		return
	}
	var in domain.ProvisioningDeliveryInput
	if !decodeStrict(w, r, &in) {
		return
	}
	response, e := h.svc.PrepareProvisioningDelivery(deviceID, tenantID(r), in)
	if e != nil {
		problem(w, 409, "PROVISIONING_DELIVERY_REJECTED")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 201, response)
}

func (h *Handler) confirmProvisioningInstallation(w http.ResponseWriter, r *http.Request, deviceID, deliveryID string) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	if !hasAnyRole(r, "ADMIN", "SERVICE", "SERVICE_TECHNICIAN") {
		problem(w, 403, "PROVISIONING_ROLE_REQUIRED")
		return
	}
	var in domain.ProvisioningInstallProof
	if !decodeStrict(w, r, &in) {
		return
	}
	response, e := h.svc.ConfirmProvisioningInstallation(deviceID, deliveryID, tenantID(r), in)
	if e != nil {
		problem(w, 409, "PROVISIONING_INSTALLATION_PROOF_REJECTED")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, response)
}
