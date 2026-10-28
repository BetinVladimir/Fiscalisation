package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"fiscalisation/fiscal-backend/internal/auth"
	"fiscalisation/fiscal-backend/internal/domain"
)

func (h *Handler) manufacturingDeviceRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	var in domain.ManufacturingDeviceInput
	if !decodeStrict(w, r, &in) {
		return
	}
	v, err := h.svc.RegisterManufacturedDevice(in)
	if err != nil {
		problem(w, 409, "MANUFACTURING_REGISTRATION_REJECTED")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 201, v)
}

func (h *Handler) platformDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	write(w, 200, map[string]any{"items": h.svc.PlatformDevices(r.URL.Query().Get("state"), r.URL.Query().Get("tenant_id"), r.URL.Query().Get("serial"))})
}

func (h *Handler) platformDevice(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(strings.TrimPrefix(r.URL.Path, "/platform/v1/devices/"), "/")
	if r.Method == http.MethodGet && !strings.Contains(p, ":") {
		v, e := h.svc.PlatformDevice(p)
		if e != nil {
			problem(w, 404, "DEVICE_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	action, id := "", p
	for _, candidate := range []string{"assign-tenant", "unassign-tenant", "suspend", "resume", "retire"} {
		suffix := ":" + candidate
		if strings.HasSuffix(p, suffix) {
			action = candidate
			id = strings.TrimSuffix(p, suffix)
			break
		}
	}
	if action == "" {
		problem(w, 404, "NOT_FOUND")
		return
	}
	var in struct {
		TenantID string `json:"tenant_id,omitempty"`
		Reason   string `json:"reason,omitempty"`
		Version  int64  `json:"version"`
	}
	if !decodeStrict(w, r, &in) {
		return
	}
	claims, _ := auth.ClaimsFrom(r.Context())
	current, err := h.svc.PlatformDevice(id)
	if err != nil {
		problem(w, 404, "DEVICE_NOT_FOUND")
		return
	}
	target, tenant := "", in.TenantID
	switch action {
	case "assign-tenant":
		target = "ASSIGNED"
	case "unassign-tenant":
		target = "MANUFACTURED"
	case "suspend":
		target = "SUSPENDED"
	case "retire":
		target = "RETIRED"
	case "resume":
		if t, _ := current["tenant_id"].(string); t != "" {
			target = "ASSIGNED"
			tenant = t
		} else {
			target = "MANUFACTURED"
		}
	}
	v, err := h.svc.TransitionPlatformDevice(id, target, tenant, in.Reason, claims.Subject, in.Version)
	if err != nil {
		problem(w, 409, "DEVICE_TRANSITION_REJECTED")
		return
	}
	write(w, 200, v)
}

func (h *Handler) deviceActivationLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	v, e := h.svc.LookupDeviceActivation(r.URL.Query().Get("user_code"))
	if e != nil {
		problem(w, 404, "ACTIVATION_REQUEST_NOT_FOUND")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, v)
}

func (h *Handler) deviceBootstrapChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	var in struct {
		DeviceInstanceID string `json:"device_instance_id"`
	}
	if !decodeStrict(w, r, &in) {
		return
	}
	v, e := h.svc.NewDeviceActivationChallenge(in.DeviceInstanceID)
	if e != nil {
		problem(w, 422, "ACTIVATION_CHALLENGE_REJECTED")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 201, v)
}

func (h *Handler) deviceBootstrapCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	var in domain.CreateDeviceActivationInput
	if !decodeStrict(w, r, &in) {
		return
	}
	v, e := h.svc.CreateDeviceActivation(in)
	if e != nil {
		problem(w, 409, "ACTIVATION_REQUEST_REJECTED")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 201, v)
}

func (h *Handler) deviceBootstrapCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/credential") {
		problem(w, 404, "NOT_FOUND")
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/device-bootstrap/v1/activation-requests/"), "/credential")
	var in struct {
		RequestSecret string `json:"request_secret"`
		Nonce         string `json:"nonce"`
		Signature     string `json:"signature"`
	}
	if id == "" || !decodeStrict(w, r, &in) {
		return
	}
	v, e := h.svc.IssueDeviceActivationCredential(id, in.RequestSecret, in.Nonce, in.Signature)
	if e != nil {
		code := "ACTIVATION_CREDENTIAL_DENIED"
		status := 403
		if strings.Contains(e.Error(), "issuer unavailable") {
			code = "ACTIVATION_CREDENTIAL_PENDING"
			status = 503
		}
		problem(w, status, code)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 201, v)
}

func (h *Handler) deviceActivationConfirmation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, ":confirm") {
		problem(w, 404, "NOT_FOUND")
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/public/v1/device-activation-requests/"), ":confirm")
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	var in domain.ConfirmDeviceActivationInput
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if id == "" || d.Decode(&in) != nil {
		problem(w, 422, "ACTIVATION_CONFIRMATION_INVALID")
		return
	}
	in.ActorSubject = actorSubject(r)
	v, e := h.svc.ConfirmDeviceActivation(id, in, tenantID(r))
	if e != nil {
		problem(w, 409, "ACTIVATION_CONFIRMATION_REJECTED")
		return
	}
	h.saveReplay(w, r, body, 200, domain.DeviceActivationPublicView(v))
}

func (h *Handler) workstation(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/workstations/"), "/")
	parts := strings.Split(p, "/")
	if (len(parts) != 2 && len(parts) != 3) || parts[0] == "" {
		problem(w, 404, "NOT_FOUND")
		return
	}
	id, action := parts[0], parts[1]
	switch {
	case len(parts) == 3 && action == "sessions" && strings.HasSuffix(parts[2], ":logout") && r.Method == http.MethodPost:
		sessionID := strings.TrimSuffix(parts[2], ":logout")
		body, ok := readBody(w, r)
		if !ok || h.replay(w, r, body) {
			return
		}
		if len(bytes.TrimSpace(body)) > 0 && string(bytes.TrimSpace(body)) != "{}" {
			problem(w, 400, "INVALID_JSON")
			return
		}
		if err := h.svc.LogoutWorkstationSession(sessionID, id, actorSubject(r), tenantID(r)); err != nil {
			problem(w, 404, "WORKSTATION_SESSION_NOT_FOUND")
			return
		}
		h.saveReplay(w, r, body, http.StatusNoContent, nil)
	case action == "sessions" && r.Method == http.MethodPost:
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if h.replay(w, r, body) {
			return
		}
		var in struct {
			OperatorCode  string `json:"operator_code"`
			AppInstanceID string `json:"app_instance_id"`
		}
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, err := h.svc.OpenWorkstationSession(id, in.OperatorCode, in.AppInstanceID, actorSubject(r), tenantID(r))
		if err != nil {
			problem(w, 409, "WORKSTATION_SESSION_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 201, v)
	case action == "readiness" && r.Method == http.MethodGet:
		v, err := h.svc.CurrentReadiness(id, tenantID(r))
		if err != nil {
			problem(w, 409, "WORKSTATION_NOT_READY")
			return
		}
		write(w, 200, v)
	case action == "readiness:refresh" && r.Method == http.MethodPost:
		v, err := h.svc.RefreshReadiness(id, tenantID(r))
		if err != nil {
			problem(w, 409, "WORKSTATION_NOT_READY")
			return
		}
		write(w, 200, v)
	case action == "clock-sync" && r.Method == http.MethodPost:
		v, err := h.svc.SyncWorkstationClock(id, tenantID(r))
		if err != nil {
			problem(w, 409, "CLOCK_SYNC_FAILED")
			return
		}
		write(w, 200, v)
	default:
		problem(w, 404, "NOT_FOUND")
	}
}

func (h *Handler) device(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/devices/"), "/")
	if strings.HasSuffix(p, ":disconnect") {
		if r.Method != http.MethodPost {
			problem(w, 405, "METHOD_NOT_ALLOWED")
			return
		}
		id := strings.TrimSuffix(p, ":disconnect")
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if h.replay(w, r, body) {
			return
		}
		if id == "" || string(bytes.TrimSpace(body)) != "{}" {
			problem(w, 422, "DEVICE_DISCONNECT_INVALID")
			return
		}
		v, e := h.svc.DisconnectSmartDevice(id, tenantID(r), actorSubject(r))
		if e != nil {
			problem(w, 409, "DEVICE_DISCONNECT_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 200, v)
		return
	}
	parts := strings.Split(p, "/")
	if len(parts) == 3 && parts[1] == "provisioning-deliveries" && strings.HasSuffix(parts[2], ":confirm") {
		h.confirmProvisioningInstallation(w, r, parts[0], strings.TrimSuffix(parts[2], ":confirm"))
		return
	}
	if len(parts) == 2 && parts[1] == "provisioning-deliveries" {
		h.provisioningDelivery(w, r, parts[0])
		return
	}
	if len(parts) == 1 {
		if r.Method == "GET" {
			v, e := h.svc.GetResource("device", parts[0], tenantID(r))
			if e != nil {
				problem(w, 404, "DEVICE_NOT_FOUND")
				return
			}
			write(w, 200, v)
			return
		}
		if r.Method == "PATCH" {
			h.resourceItem("device", "/public/v1/devices/").ServeHTTP(w, r)
			return
		}
	}
	if len(parts) == 2 && parts[1] == "readiness" && r.Method == "GET" {
		write(w, 200, h.svc.ReadinessForTenant(parts[0], tenantID(r)))
		return
	}
	if len(parts) == 2 && parts[1] == "capabilities" && r.Method == "GET" {
		v, e := h.svc.DeviceCapabilities(parts[0], tenantID(r))
		if e != nil {
			problem(w, 404, "DEVICE_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if len(parts) == 2 && parts[1] == "diagnostics" && r.Method == "GET" {
		v, e := h.svc.DeviceDiagnostics(parts[0], tenantID(r))
		if e != nil {
			problem(w, 404, "DEVICE_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if len(parts) == 2 && parts[1] == "health" && r.Method == "GET" {
		v, e := h.svc.DeviceHealth(parts[0], tenantID(r))
		if e != nil {
			problem(w, 404, "DEVICE_HEALTH_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if len(parts) == 2 && parts[1] == "activity" && r.Method == "GET" {
		if _, e := h.svc.GetResource("device", parts[0], tenantID(r)); e != nil {
			problem(w, 404, "DEVICE_NOT_FOUND")
			return
		}
		write(w, 200, map[string]any{"items": h.svc.DeviceActivity(parts[0], tenantID(r))})
		return
	}
	if len(parts) == 3 && parts[1] == "tests" && parts[2] == "printer" && r.Method == "POST" {
		if !hasAnyRole(r, "ADMIN", "SERVICE", "SERVICE_TECHNICIAN") {
			problem(w, 403, "ROLE_SERVICE_REQUIRED")
			return
		}
		v, e := h.svc.PrinterTest(parts[0], tenantID(r))
		if e != nil {
			problem(w, 409, "PRINTER_TEST_REJECTED")
			return
		}
		write(w, 202, v)
		return
	}
	if len(parts) == 2 && parts[1] == "provisioning-sessions" && r.Method == "POST" {
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if h.replay(w, r, body) {
			return
		}
		v, e := h.svc.ProvisioningSession(parts[0], tenantID(r))
		if e != nil {
			if errors.Is(e, domain.ErrNotFound) {
				problem(w, 404, "DEVICE_NOT_FOUND")
			} else {
				problem(w, 409, "DEVICE_NOT_PROVISIONABLE")
			}
			return
		}
		h.saveReplay(w, r, body, 201, v)
		return
	}
	if len(parts) == 2 && parts[1] == "activation-tokens" && r.Method == "POST" {
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		if h.replay(w, r, body) {
			return
		}
		var input struct {
			LocationID    string `json:"location_id"`
			AppInstanceID string `json:"app_instance_id"`
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || input.LocationID == "" || input.AppInstanceID == "" {
			problem(w, 422, "ACTIVATION_TOKEN_REQUEST_INVALID")
			return
		}
		v, e := h.svc.DeviceActivationToken(parts[0], input.LocationID, input.AppInstanceID, tenantID(r))
		if e != nil {
			if errors.Is(e, domain.ErrNotFound) {
				problem(w, 404, "DEVICE_OR_LOCATION_NOT_FOUND")
			} else {
				problem(w, 409, "DEVICE_NOT_ACTIVATION_ELIGIBLE")
			}
			return
		}
		h.saveReplay(w, r, body, 201, v)
		return
	}
	problem(w, 404, "NOT_FOUND")
}
