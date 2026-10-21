package api

import (
	"bytes"
	"encoding/json"
	"fiscalisation/fiscal-backend/internal/domain"
	"io"
	"net/http"
)

func (h *Handler) reconciliationAdministrator(w http.ResponseWriter, r *http.Request) {
	if !hasAnyRole(r, "ADMIN") {
		problem(w, 403, "ADMIN_REQUIRED")
		return
	}
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok || h.replay(w, r, body) {
		return
	}
	var in domain.ReconciliationAdministratorBinding
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		problem(w, 400, "INVALID_ADMINISTRATOR_BINDING")
		return
	}
	if e := h.svc.BindReconciliationAdministrator(in, tenantID(r), actorSubject(r)); e != nil {
		problem(w, 409, "ADMINISTRATOR_BINDING_REJECTED")
		return
	}
	h.saveEmptyReplay(w, r, body, 204)
}
