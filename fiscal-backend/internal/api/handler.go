package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fiscalisation/fiscal-backend/internal/auth"
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"github.com/fxamacker/cbor/v2"
)

// Handler is intentionally thin: authentication, versioning and generated
// request/response validation live at HTTP boundaries while fiscal decisions
// are delegated to domain.Service.
type Handler struct {
	svc *domain.Service
	cfg config.Config
}

func hasAnyRole(r *http.Request, allowed ...string) bool {
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		return false
	}
	for _, actual := range claims.Roles {
		for _, expected := range allowed {
			if strings.EqualFold(actual, expected) {
				return true
			}
		}
	}
	return false
}

var apiUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type webhookCreatedResponse struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Secret    string    `json:"secret"`
}

func typedWebhookCreated(v map[string]any) (webhookCreatedResponse, error) {
	var out webhookCreatedResponse
	allowed := map[string]bool{"id": true, "version": true, "url": true, "events": true, "status": true, "created_at": true, "updated_at": true, "secret": true}
	if len(v) != len(allowed) {
		return out, errors.New("webhook response property count")
	}
	for key := range v {
		if !allowed[key] {
			return out, errors.New("webhook response contains undocumented property")
		}
	}
	var ok bool
	out.ID, ok = v["id"].(string)
	if !ok || !apiUUIDPattern.MatchString(out.ID) {
		return out, errors.New("invalid webhook response id")
	}
	switch version := v["version"].(type) {
	case int64:
		out.Version = version
	case int:
		out.Version = int64(version)
	default:
		return out, errors.New("invalid webhook response version")
	}
	if out.Version < 1 {
		return out, errors.New("invalid webhook response version")
	}
	out.URL, ok = v["url"].(string)
	parsedURL, parseErr := url.Parse(out.URL)
	if !ok || parseErr != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return out, errors.New("invalid webhook response url")
	}
	rawEvents, ok := v["events"].([]any)
	if !ok || len(rawEvents) == 0 {
		return out, errors.New("invalid webhook response events")
	}
	seen := map[string]bool{}
	for _, raw := range rawEvents {
		event, valid := raw.(string)
		if !valid || event == "" || seen[event] {
			return out, errors.New("invalid webhook response event")
		}
		seen[event] = true
		out.Events = append(out.Events, event)
	}
	out.Status, ok = v["status"].(string)
	if !ok || out.Status != "ACTIVE" {
		return out, errors.New("invalid webhook response status")
	}
	out.CreatedAt, ok = v["created_at"].(time.Time)
	if !ok || out.CreatedAt.IsZero() {
		return out, errors.New("invalid webhook response created_at")
	}
	out.UpdatedAt, ok = v["updated_at"].(time.Time)
	if !ok || out.UpdatedAt.IsZero() {
		return out, errors.New("invalid webhook response updated_at")
	}
	out.Secret, ok = v["secret"].(string)
	secret, decodeErr := base64.RawURLEncoding.DecodeString(out.Secret)
	if !ok || decodeErr != nil || len(out.Secret) != 43 || len(secret) != 32 {
		return out, errors.New("invalid webhook response secret")
	}
	return out, nil
}

func NewHandler(s *domain.Service, c config.Config) http.Handler {
	h := &Handler{s, c}
	m := http.NewServeMux()
	m.HandleFunc("/connectivity/ping", h.ping)
	m.HandleFunc("/public/v1/connectivity/ping", h.ping)
	m.HandleFunc("/livez", h.live)
	m.HandleFunc("/readyz", h.live)
	m.HandleFunc("/healthz", h.live)
	m.HandleFunc("/public/v1/version", h.version)
	m.HandleFunc("/public/v1/sales", h.sales)
	m.HandleFunc("/public/v1/sales:open-with-line", h.openSaleWithLine)
	m.HandleFunc("/public/v1/sales/", h.sale)
	m.HandleFunc("/public/v1/operations/", h.operation)
	m.HandleFunc("/public/v1/operations", h.operations)
	m.HandleFunc("/public/v1/devices/", h.device)
	m.HandleFunc("/public/v1/devices", h.resourceCollection("device"))
	m.HandleFunc("/public/v1/organizations", h.organization)
	m.HandleFunc("/public/v1/locations", h.resourceCollection("location"))
	m.HandleFunc("/public/v1/locations/", h.resourceItem("location", "/public/v1/locations/"))
	m.HandleFunc("/public/v1/registers", h.resourceCollection("register"))
	m.HandleFunc("/public/v1/shifts", h.shifts)
	m.HandleFunc("/public/v1/shifts/", h.shift)
	m.HandleFunc("/public/v1/registers/", h.register)
	m.HandleFunc("/public/v1/workstations/", h.workstation)
	m.HandleFunc("/public/v1/connectivity-probes/", h.connectivityProbe)
	m.HandleFunc("/public/v1/operators", h.resourceCollection("operator"))
	m.HandleFunc("/public/v1/operators/", h.resourceItem("operator", "/public/v1/operators/"))
	m.HandleFunc("/public/v1/reports", h.reports)
	m.HandleFunc("/public/v1/reports/", h.report)
	m.HandleFunc("/public/v1/audit-events", h.auditEvents)
	m.HandleFunc("/public/v1/exports", h.exports)
	m.HandleFunc("/public/v1/exports/periodized", h.periodizedExports)
	m.HandleFunc("/public/v1/exports/", h.export)
	m.HandleFunc("/public/v1/tax-groups", h.taxGroups)
	m.HandleFunc("/public/v1/country-policy", h.countryPolicy)
	m.HandleFunc("/public/v1/webhook-endpoints", h.webhookEndpoints)
	m.HandleFunc("/public/v1/webhook-endpoints/", h.webhookEndpoint)
	m.HandleFunc("/public/v1/ble-sessions/", h.bleSession)
	m.HandleFunc("/public/v1/edge-sync/batches", h.sync)
	m.HandleFunc("/public/v1/device-activation-requests/", h.deviceActivationConfirmation)
	m.HandleFunc("/public/v1/device-activation-requests:lookup", h.deviceActivationLookup)
	m.HandleFunc("/device-bootstrap/v1/challenges", h.deviceBootstrapChallenge)
	m.HandleFunc("/device-bootstrap/v1/activation-requests", h.deviceBootstrapCreate)
	m.HandleFunc("/device-bootstrap/v1/activation-requests/", h.deviceBootstrapCredential)
	m.HandleFunc("/platform/v1/manufacturing/devices:register", h.manufacturingDeviceRegister)
	m.HandleFunc("/platform/v1/devices", h.platformDevices)
	m.HandleFunc("/platform/v1/devices/", h.platformDevice)
	oidc := auth.NewOIDCVerifier(c.OIDCIssuer, c.OIDCAudience, c.OIDCJWKSURL)
	business := auth.MiddlewareWithOIDC(c.AuthHMACKey, oidc, rateLimit(600, time.Minute, recoverer(version(c.APIVersion, enforceSuccessResponses(fiscalIdempotency(s, m))))))
	pingLimiter := &requestLimiter{limit: 120, window: time.Minute, now: time.Now, entries: make(map[string]rateWindow)}
	pingRoute := pingLimiter.middlewareAll(http.HandlerFunc(h.ping))
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/connectivity/ping" || r.URL.Path == "/public/v1/connectivity/ping" {
			pingRoute.ServeHTTP(w, r)
			return
		}
		business.ServeHTTP(w, r)
	})
	return cors(c.CORSAllowedOrigins, root)
}

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

func decodeStrict(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		problem(w, 400, "INVALID_BODY")
		return false
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		problem(w, 422, "REQUEST_INVALID")
		return false
	}
	return true
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

type fiscalIdempotencyOwnerKey struct{}

type fiscalCapturedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *fiscalCapturedResponse) Header() http.Header { return w.header }
func (w *fiscalCapturedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *fiscalCapturedResponse) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func fiscalIdempotency(s *domain.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/public/v1") || (r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete) {
			next.ServeHTTP(w, r)
			return
		}
		if n := len(r.Header.Get("Idempotency-Key")); n < 16 || n > 255 {
			problem(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			problem(w, http.StatusBadRequest, "INVALID_BODY")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		hash := fiscalIdempotencyFingerprint(r, body)
		old, owner, err := s.ClaimReplay(replayKey(r), hash)
		if err != nil {
			if strings.Contains(err.Error(), "mismatch") {
				problem(w, http.StatusConflict, "IDEMPOTENCY_MISMATCH")
			} else {
				problem(w, http.StatusServiceUnavailable, "IDEMPOTENCY_RESERVATION_FAILED")
			}
			return
		}
		if !owner {
			if old.Pending {
				applyFiscalReplayHeaders(w.Header(), old.Headers, old.ContentType)
				w.Header().Set("Retry-After", "1")
				if old.Status >= 500 && len(old.Body) > 0 {
					w.Header().Set("Idempotency-Replayed", "true")
					if w.Header().Get("Content-Type") == "" {
						w.Header().Set("Content-Type", "application/json")
					}
					w.WriteHeader(old.Status)
					_, _ = w.Write(old.Body)
				} else {
					problem(w, http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS")
				}
				return
			}
			w.Header().Set("Idempotency-Replayed", "true")
			applyFiscalReplayHeaders(w.Header(), old.Headers, old.ContentType)
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(old.Status)
			_, _ = w.Write(old.Body)
			return
		}
		cw := &fiscalCapturedResponse{header: make(http.Header)}
		r = r.WithContext(context.WithValue(r.Context(), fiscalIdempotencyOwnerKey{}, true))
		next.ServeHTTP(cw, r)
		if cw.status == 0 {
			cw.status = http.StatusOK
		}
		if cw.status < 500 {
			if err = s.PutReplay(replayKey(r), domain.ReplayRecord{Hash: hash, Status: cw.status, Body: append([]byte(nil), cw.body.Bytes()...), ContentType: cw.header.Get("Content-Type"), Headers: fiscalReplayHeaders(cw.header)}); err != nil {
				problem(w, http.StatusInternalServerError, "IDEMPOTENCY_PERSIST_FAILED")
				return
			}
		} else {
			cw.header.Set("Retry-After", "1")
			if err = s.PutAmbiguousReplay(replayKey(r), domain.ReplayRecord{Hash: hash, Status: cw.status, Body: append([]byte(nil), cw.body.Bytes()...), ContentType: cw.header.Get("Content-Type"), Headers: fiscalReplayHeaders(cw.header), Pending: true}); err != nil {
				problem(w, http.StatusInternalServerError, "IDEMPOTENCY_PERSIST_FAILED")
				return
			}
		}
		for key, values := range cw.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(cw.status)
		_, _ = w.Write(cw.body.Bytes())
	})
}

var fiscalReplayableHeader = map[string]bool{"Content-Type": true, "Etag": true, "Location": true, "Retry-After": true, "Cache-Control": true}

func fiscalReplayHeaders(source http.Header) map[string][]string {
	result := map[string][]string{}
	for key, values := range source {
		canonical := http.CanonicalHeaderKey(key)
		if fiscalReplayableHeader[canonical] {
			result[canonical] = append([]string(nil), values...)
		}
	}
	return result
}

func applyFiscalReplayHeaders(target http.Header, headers map[string][]string, legacyContentType string) {
	for key, values := range headers {
		canonical := http.CanonicalHeaderKey(key)
		if !fiscalReplayableHeader[canonical] {
			continue
		}
		for _, value := range values {
			target.Add(canonical, value)
		}
	}
	if target.Get("Content-Type") == "" && legacyContentType != "" {
		target.Set("Content-Type", legacyContentType)
	}
}

func fiscalIdempotencyFingerprint(r *http.Request, body []byte) string {
	h := sha256.New()
	issuer, subject := "", ""
	if claims, ok := auth.ClaimsFrom(r.Context()); ok {
		issuer, subject = claims.Issuer, claims.Subject
	}
	for _, value := range []string{r.URL.Query().Encode(), r.Header.Get("If-Match"), r.Header.Get("X-App-Instance-Id"), issuer, subject} {
		_, _ = fmt.Fprintf(h, "%d:", len(value))
		_, _ = h.Write([]byte(value))
	}
	_, _ = fmt.Fprintf(h, "%d:", len(body))
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
func (h *Handler) webhookEndpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok || h.replay(w, r, body) {
		return
	}
	var in map[string]any
	if json.Unmarshal(body, &in) != nil {
		problem(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}
	v, err := h.svc.CreateWebhookEndpoint(tenantID(r), in)
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "WEBHOOK_ENDPOINT_REJECTED")
		return
	}
	typed, err := typedWebhookCreated(v)
	if err != nil {
		problem(w, http.StatusInternalServerError, "RESPONSE_CONTRACT_VIOLATION")
		return
	}
	h.saveReplay(w, r, body, http.StatusCreated, typed)
}

func (h *Handler) webhookEndpoint(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/webhook-endpoints/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" || len(parts) > 2 {
		problem(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		v, err := h.svc.GetWebhookEndpoint(id, tenantID(r))
		if err != nil {
			problem(w, http.StatusNotFound, "WEBHOOK_ENDPOINT_NOT_FOUND")
			return
		}
		write(w, http.StatusOK, v)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPatch {
		body, ok := readBody(w, r)
		if !ok || h.replay(w, r, body) {
			return
		}
		expected, ok := ifMatch(r)
		if !ok {
			problem(w, http.StatusPreconditionRequired, "IF_MATCH_REQUIRED")
			return
		}
		var in map[string]any
		if json.Unmarshal(body, &in) != nil {
			problem(w, http.StatusBadRequest, "INVALID_JSON")
			return
		}
		v, err := h.svc.UpdateWebhookEndpoint(id, tenantID(r), expected, in)
		if err != nil {
			problem(w, http.StatusConflict, "WEBHOOK_ENDPOINT_UPDATE_REJECTED")
			return
		}
		h.saveReplay(w, r, body, http.StatusOK, v)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		body := []byte{}
		if h.replay(w, r, body) {
			return
		}
		if err := h.svc.DisableWebhookEndpoint(id, tenantID(r)); err != nil {
			problem(w, http.StatusNotFound, "WEBHOOK_ENDPOINT_NOT_FOUND")
			return
		}
		h.saveEmptyReplay(w, r, body, http.StatusNoContent)
		return
	}
	if len(parts) == 2 && parts[1] == "rotate-secret" && r.Method == http.MethodPost {
		body := []byte{}
		if h.replay(w, r, body) {
			return
		}
		v, err := h.svc.RotateWebhookSecret(id, tenantID(r))
		if err != nil {
			problem(w, http.StatusNotFound, "WEBHOOK_ENDPOINT_NOT_FOUND")
			return
		}
		h.saveReplay(w, r, body, http.StatusOK, v)
		return
	}
	problem(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
}
func (h *Handler) taxGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		problem(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	at := time.Now().UTC()
	if raw := r.URL.Query().Get("effective_at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			problem(w, http.StatusBadRequest, "INVALID_EFFECTIVE_AT")
			return
		}
		at = parsed
	}
	groups, err := h.svc.TaxGroups(at)
	if err != nil {
		problem(w, http.StatusNotFound, "POLICY_NOT_EFFECTIVE")
		return
	}
	write(w, http.StatusOK, groups)
}
func (h *Handler) countryPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		problem(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	v, err := h.svc.CountryPolicy(time.Now().UTC())
	if err != nil {
		problem(w, http.StatusNotFound, "POLICY_NOT_EFFECTIVE")
		return
	}
	write(w, http.StatusOK, v)
}
func (h *Handler) organization(w http.ResponseWriter, r *http.Request) {
	tenant := tenantID(r)
	if r.Method == "GET" {
		v, e := h.svc.Organization(tenant)
		if e != nil {
			problem(w, 404, "ORGANIZATION_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if r.Method != "PATCH" {
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
	var in map[string]any
	if json.Unmarshal(body, &in) != nil {
		problem(w, 400, "INVALID_JSON")
		return
	}
	expected, ok := ifMatch(r)
	if !ok {
		problem(w, 428, "IF_MATCH_REQUIRED")
		return
	}
	v, e := h.svc.UpsertOrganization(tenant, expected, in)
	if e != nil {
		problem(w, 409, "ORGANIZATION_REJECTED")
		return
	}
	h.saveReplay(w, r, body, 200, v)
}
func (h *Handler) resourceCollection(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			page, err := paginate(r, h.svc.ListResources(kind, tenantID(r)))
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
		var in map[string]any
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.CreateResource(kind, tenantID(r), in)
		if e != nil {
			problem(w, 422, "RESOURCE_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 201, v)
	}
}
func (h *Handler) resourceItem(kind, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
		if id == "" || strings.Contains(id, "/") {
			problem(w, 404, "NOT_FOUND")
			return
		}
		if r.Method == "GET" {
			v, e := h.svc.GetResource(kind, id, tenantID(r))
			if e != nil {
				problem(w, 404, "NOT_FOUND")
				return
			}
			write(w, 200, v)
			return
		}
		if r.Method != "PATCH" {
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
		expected, ok := ifMatch(r)
		if !ok {
			problem(w, 428, "IF_MATCH_REQUIRED")
			return
		}
		var in map[string]any
		if json.Unmarshal(body, &in) != nil {
			problem(w, 400, "INVALID_JSON")
			return
		}
		v, e := h.svc.UpdateResource(kind, id, tenantID(r), expected, in)
		if e != nil {
			problem(w, 409, "RESOURCE_UPDATE_REJECTED")
			return
		}
		h.saveReplay(w, r, body, 200, v)
	}
}
func ifMatch(r *http.Request) (int64, bool) {
	v := strings.Trim(r.Header.Get("If-Match"), `"`)
	if v == "" {
		return 0, false
	}
	n, e := strconv.ParseInt(v, 10, 64)
	return n, e == nil
}
func (h *Handler) live(w http.ResponseWriter, _ *http.Request) {
	write(w, 200, map[string]any{"status": "ok"})
}
func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodHead && r.Method != http.MethodGet {
		w.Header().Set("Allow", "HEAD, GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-BeeFiscal-Ping", "1")
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	write(w, 200, map[string]string{"api": h.cfg.APIVersion, "build": "mvp-dev", "policy": "BG-2026-EUR", "schema": "1"})
}
func (h *Handler) sales(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
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
	h.saveReplay(w, r, body, 202, v)
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
func (h *Handler) reports(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	items := h.svc.Reports(tenantID(r))
	if register := r.URL.Query().Get("register_id"); register != "" {
		filtered := items[:0]
		for _, item := range items {
			if item["register_id"] == register {
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
}
func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	p := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/reports/"), "/"), "/")
	if len(p) == 1 && r.Method == "GET" {
		v, e := h.svc.Report(p[0], tenantID(r))
		if e != nil {
			problem(w, 404, "REPORT_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if len(p) == 3 && p[1] == "artifacts" && r.Method == "GET" {
		v, e := h.svc.ReportArtifact(p[0], p[2], tenantID(r))
		if e != nil {
			problem(w, 404, "ARTIFACT_NOT_FOUND")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		_, _ = w.Write(v)
		return
	}
	problem(w, 404, "NOT_FOUND")
}
func (h *Handler) auditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	from, err := optionalTime(r.URL.Query().Get("from"))
	if err != nil {
		problem(w, 400, "INVALID_AUDIT_PERIOD")
		return
	}
	to, err := optionalTime(r.URL.Query().Get("to"))
	if err != nil || (!from.IsZero() && !to.IsZero() && to.Before(from)) {
		problem(w, 400, "INVALID_AUDIT_PERIOD")
		return
	}
	items := h.svc.AuditEvents(tenantID(r))
	filtered := items[:0]
	for _, item := range items {
		if actor := r.URL.Query().Get("actor_id"); actor != "" && item.ActorID != actor {
			continue
		}
		if action := r.URL.Query().Get("action"); action != "" && item.Action != action {
			continue
		}
		if objectType := r.URL.Query().Get("object_type"); objectType != "" && item.ObjectType != objectType {
			continue
		}
		if objectID := r.URL.Query().Get("object_id"); objectID != "" && item.ObjectID != objectID {
			continue
		}
		if unp := r.URL.Query().Get("unp"); unp != "" && item.UNP != unp {
			continue
		}
		if !from.IsZero() && item.OccurredAt.Before(from) || !to.IsZero() && item.OccurredAt.After(to) {
			continue
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
func (h *Handler) exports(w http.ResponseWriter, r *http.Request) {
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
	var in domain.ExportRequest
	if json.Unmarshal(body, &in) != nil {
		problem(w, 400, "INVALID_JSON")
		return
	}
	v, e := h.svc.CreateExport(in, tenantID(r))
	if e != nil {
		problem(w, 422, "EXPORT_REJECTED")
		return
	}
	h.saveReplay(w, r, body, 202, v)
}

func (h *Handler) periodizedExports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok || h.replay(w, r, body) {
		return
	}
	var in domain.ExportRequest
	if json.Unmarshal(body, &in) != nil {
		problem(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}
	v, err := h.svc.CreatePeriodizedExport(in, tenantID(r))
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "PERIODIZED_EXPORT_REJECTED")
		return
	}
	h.saveReplay(w, r, body, http.StatusAccepted, v)
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/exports/"), "/"), "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] == "periods" {
		v, err := h.svc.ExportPeriods(parts[0], tenantID(r))
		if err != nil {
			problem(w, http.StatusNotFound, "EXPORT_NOT_FOUND")
			return
		}
		write(w, http.StatusOK, v)
		return
	}
	if len(parts) == 3 && parts[0] != "" && parts[1] == "artifacts" && parts[2] != "" {
		artifact, media, err := h.svc.ExportPeriodArtifact(parts[0], parts[2], tenantID(r))
		if err != nil {
			artifact, media, err = h.svc.ExportArtifactByID(parts[0], parts[2], tenantID(r))
			if err != nil {
				problem(w, http.StatusNotFound, "EXPORT_ARTIFACT_NOT_FOUND")
				return
			}
		}
		w.Header().Set("Content-Type", media)
		w.Header().Set("Content-Disposition", `attachment; filename="compliance-export-`+parts[2]+exportExtension(media)+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(artifact)
		return
	}
	if len(parts) != 1 || parts[0] == "" {
		problem(w, http.StatusNotFound, "EXPORT_NOT_FOUND")
		return
	}
	id := parts[0]
	v, e := h.svc.Export(id, tenantID(r))
	if e != nil {
		problem(w, 404, "EXPORT_NOT_FOUND")
		return
	}
	write(w, 200, v)
}
func exportExtension(media string) string {
	switch media {
	case "application/json":
		return ".json"
	case "text/csv":
		return ".csv"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	default:
		return ".bin"
	}
}
func (h *Handler) connectivityProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/connectivity-probes/"), "/")
	v, e := h.svc.GetConnectivityProbe(id, tenantID(r))
	if e != nil {
		problem(w, 404, "PROBE_NOT_FOUND")
		return
	}
	write(w, 200, v)
}
func (h *Handler) bleSession(w http.ResponseWriter, r *http.Request) {
	p := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/ble-sessions/"), "/"), "/")
	if len(p) != 2 || r.Method != "POST" {
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
	case "refresh":
		v, e := h.svc.RefreshBLE(p[0], tenantID(r), actorSubject(r))
		if e != nil {
			problem(w, 409, "BLE_SESSION_INACTIVE")
			return
		}
		h.saveReplay(w, r, body, 200, v)
	case "revoke":
		if e := h.svc.RevokeBLE(p[0], tenantID(r), actorSubject(r)); e != nil {
			problem(w, 404, "BLE_SESSION_NOT_FOUND")
			return
		}
		h.saveEmptyReplay(w, r, body, http.StatusNoContent)
	default:
		problem(w, 404, "NOT_FOUND")
	}
}
func (h *Handler) sync(w http.ResponseWriter, r *http.Request) {
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
	var in domain.EdgeSyncBatch
	var decodeErr error
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/cbor") {
		decodeErr = cbor.Unmarshal(body, &in)
	} else {
		decodeErr = json.Unmarshal(body, &in)
	}
	if decodeErr != nil {
		problem(w, 422, "INVALID_SYNC_BATCH")
		return
	}
	v, e := h.svc.SyncBatchForTenant(tenantID(r), in)
	if e != nil {
		problem(w, 409, "SYNC_GAP_OR_SIGNING_ERROR")
		return
	}
	h.saveReplay(w, r, body, 200, v)
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
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if n := len(r.Header.Get("Idempotency-Key")); n < 16 || n > 255 {
		problem(w, 400, "IDEMPOTENCY_KEY_REQUIRED")
		return nil, false
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		problem(w, 400, "INVALID_BODY")
		return nil, false
	}
	return b, true
}

func (h *Handler) replay(w http.ResponseWriter, r *http.Request, b []byte) bool {
	if owner, _ := r.Context().Value(fiscalIdempotencyOwnerKey{}).(bool); owner {
		return false
	}
	k := replayKey(r)
	v, ok := h.svc.Replay(k)
	if !ok {
		return false
	}
	if v.Hash != fiscalIdempotencyFingerprint(r, b) {
		problem(w, 409, "IDEMPOTENCY_MISMATCH")
		return true
	}
	w.Header().Set("Idempotency-Replayed", "true")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(v.Status)
	_, _ = w.Write(v.Body)
	return true
}
func (h *Handler) saveReplay(w http.ResponseWriter, r *http.Request, b []byte, status int, v any) {
	out, _ := json.Marshal(v)
	k := replayKey(r)
	if e := h.svc.PutReplay(k, domain.ReplayRecord{Hash: fiscalIdempotencyFingerprint(r, b), Status: status, Body: out, ContentType: "application/json"}); e != nil {
		problem(w, 500, "IDEMPOTENCY_PERSIST_FAILED")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(out)
}
func (h *Handler) saveEmptyReplay(w http.ResponseWriter, r *http.Request, b []byte, status int) {
	k := replayKey(r)
	if e := h.svc.PutReplay(k, domain.ReplayRecord{Hash: fiscalIdempotencyFingerprint(r, b), Status: status, Body: []byte{}}); e != nil {
		problem(w, 500, "IDEMPOTENCY_PERSIST_FAILED")
		return
	}
	w.WriteHeader(status)
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func tenantID(r *http.Request) string {
	if c, ok := auth.ClaimsFrom(r.Context()); ok {
		return c.TenantID
	}
	return ""
}
func actorSubject(r *http.Request) string {
	if c, ok := auth.ClaimsFrom(r.Context()); ok {
		return c.Subject
	}
	return ""
}
func replayKey(r *http.Request) string {
	return tenantID(r) + " " + r.Method + " " + r.URL.Path + " " + r.Header.Get("Idempotency-Key")
}

func paginate[T any](r *http.Request, all []T) (map[string]any, error) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			return nil, errors.New("invalid limit")
		}
		limit = parsed
	}
	start := 0
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return nil, errors.New("invalid cursor")
		}
		if _, err = fmt.Sscanf(string(decoded), "offset:%d", &start); err != nil || start < 0 || start > len(all) {
			return nil, errors.New("invalid cursor")
		}
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	hasMore := end < len(all)
	var next any
	if hasMore {
		next = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("offset:%d", end)))
	}
	return map[string]any{"items": all[start:end], "page": map[string]any{"next_cursor": next, "has_more": hasMore}}, nil
}

func optionalTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}
func (h *Handler) authorizeSale(w http.ResponseWriter, r *http.Request, id string) bool {
	tenant := tenantID(r)
	if tenant == "" {
		return true
	}
	_, e := h.svc.GetSaleForTenant(id, tenant)
	if e != nil {
		problem(w, 404, "SALE_NOT_FOUND")
		return false
	}
	return true
}
func problem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "urn:beefiscal:error:" + strings.ToLower(code), "title": code, "status": status, "code": code, "retryable": false, "trace_id": time.Now().UTC().Format("20060102150405.000000000")})
}
func version(v string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/public/v1") && r.Header.Get("X-Api-Version") != v {
			problem(w, 400, "API_VERSION_REQUIRED")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				problem(w, 500, "INTERNAL")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func cors(allowed string, next http.Handler) http.Handler {
	set := map[string]bool{}
	for _, v := range strings.Split(allowed, ",") {
		set[strings.TrimSpace(v)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && set[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match, X-Api-Version, X-Tenant-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			if origin == "" || !set[origin] {
				problem(w, 403, "ORIGIN_NOT_ALLOWED")
				return
			}
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func SignWebhook(key string, b []byte) string {
	m := hmac.New(sha256.New, []byte(key))
	_, _ = io.Copy(m, bytes.NewReader(b))
	return hex.EncodeToString(m.Sum(nil))
}

var _ = errors.Is
