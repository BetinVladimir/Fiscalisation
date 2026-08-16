package integration

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"fiscalisation/fiscal-backend/internal/auth"
)

type HTTPHandler struct {
	Service       *Service
	PublicBaseURL string
}

func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	code := "INVALID_REQUEST"
	if errors.Is(err, ErrUnauthorized) {
		status = http.StatusUnauthorized
		code = "CREDENTIAL_INVALID"
	} else if errors.Is(err, ErrConflict) {
		status = http.StatusConflict
		code = "INTEGRATION_CONFLICT"
	} else if errors.Is(err, ErrNotFound) {
		status = http.StatusNotFound
		code = "NOT_FOUND"
	} else if errors.Is(err, ErrRateLimited) {
		status = http.StatusTooManyRequests
		code = "RATE_LIMITED"
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": err.Error()}})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple json values")
	}
	return nil
}
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/public/v1/app-auth/tenants" && r.Method == http.MethodGet {
		items, e := h.Service.AppTenantsForAccess(r.Context(), bearer(r))
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	if path == "/public/v1/app-auth/challenges" && r.Method == http.MethodPost {
		var in struct {
			Email         string `json:"email"`
			Language      string `json:"language"`
			AppInstanceID string `json:"app_instance_id"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.StartAppChallenge(r.Context(), in.Email, in.AppInstanceID, r.RemoteAddr)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusAccepted, out)
		return
	}
	if path == "/public/v1/app-auth/challenges:verify" && r.Method == http.MethodPost {
		var in struct {
			Code          string `json:"code"`
			AppInstanceID string `json:"app_instance_id"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.VerifyAppChallenge(r.Context(), bearer(r), in.Code, in.AppInstanceID)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if path == "/public/v1/app-auth/tenant-session" && r.Method == http.MethodPost {
		var in struct {
			TenantID      string `json:"tenant_id"`
			AppInstanceID string `json:"app_instance_id"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.SelectAppTenant(r.Context(), bearer(r), in.TenantID, in.AppInstanceID)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if path == "/public/v1/app-auth/refresh" && r.Method == http.MethodPost {
		var in struct {
			RefreshToken  string `json:"refresh_token"`
			AppInstanceID string `json:"app_instance_id"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.RotateAppSession(r.Context(), in.RefreshToken, in.AppInstanceID, "")
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if path == "/public/v1/app-auth/sessions:switch-tenant" && r.Method == http.MethodPost {
		var in struct {
			RefreshToken  string `json:"refresh_token"`
			TenantID      string `json:"tenant_id"`
			AppInstanceID string `json:"app_instance_id"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.RotateAppSession(r.Context(), in.RefreshToken, in.AppInstanceID, in.TenantID)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if path == "/public/v1/app-auth/logout" && r.Method == http.MethodPost {
		var in struct {
			RefreshToken  string `json:"refresh_token"`
			AppInstanceID string `json:"app_instance_id"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		if e := h.Service.LogoutAppSession(r.Context(), in.RefreshToken, in.AppInstanceID); e != nil {
			writeError(w, e)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if strings.HasPrefix(path, "/platform/v1/external-systems") || strings.HasPrefix(path, "/platform/v1/webhook-deliveries/") || strings.HasPrefix(path, "/platform/v1/enrollment-conflicts") || path == "/platform/v1/integration-metrics" {
		h.platform(w, r, path)
		return
	}
	if path == "/integration/v1/enrollments" && r.Method == http.MethodPost {
		var in EnrollmentRequest
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.StartEnrollment(r.Context(), bearer(r), r.Header.Get("Idempotency-Key"), r.RemoteAddr, in)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusAccepted, out)
		return
	}
	if path == "/integration/v1/enrollments:verify" && r.Method == http.MethodPost {
		var in struct {
			Code string `json:"code"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.VerifyEnrollment(r.Context(), bearer(r), in.Code, r.Header.Get("Idempotency-Key"))
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if path == "/integration/v1/credentials:recover" && r.Method == http.MethodPost {
		var in CredentialRecoveryRequest
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.StartCredentialRecovery(r.Context(), bearer(r), r.Header.Get("Idempotency-Key"), in)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusAccepted, out)
		return
	}
	if path == "/integration/v1/credentials:recover-verify" && r.Method == http.MethodPost {
		var in struct {
			Code string `json:"code"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.VerifyCredentialRecovery(r.Context(), bearer(r), in.Code, r.Header.Get("Idempotency-Key"))
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	p, e := h.Service.AuthenticateTenant(r.Context(), bearer(r))
	if e != nil {
		writeError(w, e)
		return
	}
	if path == "/integration/v1/credentials:rotate" && r.Method == http.MethodPost {
		out, e := h.Service.RotateTenantCredential(r.Context(), p, r.Header.Get("Idempotency-Key"))
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if path == "/integration/v1/credentials:revoke" && r.Method == http.MethodPost {
		e := h.Service.RevokeTenantCredential(r.Context(), p, r.Header.Get("Idempotency-Key"), r.Header.Get("BeeFiscal-Source-Actor-Type"), r.Header.Get("BeeFiscal-Source-Actor-Id"), r.Header.Get("BeeFiscal-Source-Actor-Session-Id"))
		if e != nil {
			writeError(w, e)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if path == "/integration/v1/binding" && r.Method == http.MethodPatch {
		var in struct {
			SourceCompanyID string `json:"source_company_id"`
			ExpectedVersion int64  `json:"expected_version"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		e := h.Service.UpdateSourceCompanyID(r.Context(), p, in.SourceCompanyID, in.ExpectedVersion, r.Header.Get("BeeFiscal-Source-Actor-Type"), r.Header.Get("BeeFiscal-Source-Actor-Id"), r.Header.Get("BeeFiscal-Source-Actor-Session-Id"), r.Header.Get("Idempotency-Key"))
		if e != nil {
			writeError(w, e)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if strings.HasPrefix(path, "/integration/v1/operations/") && r.Method == http.MethodGet {
		out, e := h.Service.Operation(r.Context(), p, strings.TrimPrefix(path, "/integration/v1/operations/"))
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	resource, sourceID, ok := resourcePath(path)
	if !ok || (r.Method != http.MethodPut && r.Method != http.MethodDelete) {
		http.NotFound(w, r)
		return
	}
	version, e := strconv.ParseInt(r.Header.Get("Source-Version"), 10, 64)
	if e != nil {
		writeError(w, errors.New("invalid Source-Version"))
		return
	}
	body := []byte(`{}`)
	if r.Method == http.MethodPut {
		body, e = io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if e != nil {
			writeError(w, e)
			return
		}
	}
	out, e := h.Service.AcceptResource(r.Context(), p, r.Method, resource, sourceID, r.Header.Get("Idempotency-Key"), version, r.Header.Get("BeeFiscal-Source-Actor-Type"), r.Header.Get("BeeFiscal-Source-Actor-Id"), r.Header.Get("BeeFiscal-Source-Actor-Session-Id"), body, h.PublicBaseURL)
	if e != nil {
		h.Service.AuditRejectedMutation(r.Context(), p, resource, sourceID, r.Header.Get("Idempotency-Key"), r.Header.Get("BeeFiscal-Source-Actor-Type"), r.Header.Get("BeeFiscal-Source-Actor-Id"), e)
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}
func resourcePath(path string) (string, string, bool) {
	base := "/integration/v1/"
	if !strings.HasPrefix(path, base) {
		return "", "", false
	}
	tail := strings.TrimPrefix(path, base)
	parts := strings.Split(tail, "/")
	if len(parts) == 1 && parts[0] == "organization" {
		return "organization", "organization", true
	}
	if len(parts) != 2 || parts[1] == "" {
		return "", "", false
	}
	singular := map[string]string{"locations": "location", "registers": "register", "operators": "operator"}[parts[0]]
	return singular, parts[1], singular != ""
}
func platformActor(r *http.Request) (string, bool) {
	c, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		return "", false
	}
	for _, role := range c.Roles {
		if role == "PLATFORM_SECURITY_ADMIN" || role == "PLATFORM_INTEGRATION_ADMIN" {
			return c.Subject, true
		}
	}
	return "", false
}
func (h *HTTPHandler) platform(w http.ResponseWriter, r *http.Request, path string) {
	actor, ok := platformActor(r)
	if !ok {
		writeError(w, ErrUnauthorized)
		return
	}
	base := "/platform/v1/external-systems"
	if path == "/platform/v1/integration-metrics" && r.Method == http.MethodGet {
		out, e := h.Service.IntegrationMetrics(r.Context())
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if path == "/platform/v1/enrollment-conflicts" && r.Method == http.MethodGet {
		items, e := h.Service.EnrollmentConflicts(r.Context())
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	if strings.HasPrefix(path, "/platform/v1/enrollment-conflicts/") && strings.HasSuffix(path, ":resolve") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/platform/v1/enrollment-conflicts/"), ":resolve")
		var in struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		if e := h.Service.ResolveEnrollmentConflict(r.Context(), id, in.Decision, in.Reason, actor, r.Header.Get("Idempotency-Key")); e != nil {
			writeError(w, e)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if strings.HasPrefix(path, "/platform/v1/webhook-deliveries/") && strings.HasSuffix(path, ":requeue") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/platform/v1/webhook-deliveries/"), ":requeue")
		if e := h.Service.RequeueDelivery(r.Context(), id, actor, r.Header.Get("Idempotency-Key")); e != nil {
			writeError(w, e)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if path == base {
		if r.Method == http.MethodGet {
			items, e := h.Service.ListSystems(r.Context())
			if e != nil {
				writeError(w, e)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
			return
		}
		if r.Method == http.MethodPost {
			var in struct {
				Code          string   `json:"code"`
				DisplayName   string   `json:"display_name"`
				WebhookURL    string   `json:"webhook_url"`
				WebhookEvents []string `json:"webhook_events"`
			}
			if e := decode(r, &in); e != nil {
				writeError(w, e)
				return
			}
			system, key, e := h.Service.CreateSystem(r.Context(), actor, r.Header.Get("Idempotency-Key"), in.Code, in.DisplayName, in.WebhookURL, in.WebhookEvents)
			if e != nil {
				writeError(w, e)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"system": system, "bootstrap_token": key})
			return
		}
	}
	rest := strings.TrimPrefix(path, base+"/")
	pathParts := strings.Split(rest, "/")
	if len(pathParts) == 2 && r.Method == http.MethodGet {
		var items []map[string]any
		var e error
		switch pathParts[1] {
		case "audit-events":
			items, e = h.Service.SystemAudit(r.Context(), pathParts[0])
		case "tenant-bindings":
			items, e = h.Service.SystemBindings(r.Context(), pathParts[0])
		case "webhook-deliveries":
			items, e = h.Service.SystemDeliveries(r.Context(), pathParts[0])
		default:
			http.NotFound(w, r)
			return
		}
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	if len(pathParts) == 1 && !strings.Contains(rest, ":") && r.Method == http.MethodPatch {
		var in struct {
			DisplayName   string   `json:"display_name"`
			WebhookURL    string   `json:"webhook_url"`
			WebhookEvents []string `json:"webhook_events"`
			Version       int64    `json:"version"`
		}
		if e := decode(r, &in); e != nil {
			writeError(w, e)
			return
		}
		out, e := h.Service.UpdateSystem(r.Context(), rest, actor, r.Header.Get("Idempotency-Key"), in.DisplayName, in.WebhookURL, in.WebhookEvents, in.Version)
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 2 || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "rotate-key":
		key, e := h.Service.RotateSystemKey(r.Context(), parts[0], actor, r.Header.Get("Idempotency-Key"))
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"bootstrap_token": key})
	case "suspend":
		if e := h.Service.SetSystemStatus(r.Context(), parts[0], "SUSPENDED", actor, r.Header.Get("Idempotency-Key")); e != nil {
			writeError(w, e)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "resume":
		if e := h.Service.SetSystemStatus(r.Context(), parts[0], "ACTIVE", actor, r.Header.Get("Idempotency-Key")); e != nil {
			writeError(w, e)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}
