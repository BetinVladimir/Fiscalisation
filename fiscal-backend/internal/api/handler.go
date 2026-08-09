package api

import (
	"bytes"
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

type Handler struct {
	svc *domain.Service
	cfg config.Config
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
	m.HandleFunc("/livez", h.live)
	m.HandleFunc("/readyz", h.live)
	m.HandleFunc("/healthz", h.live)
	m.HandleFunc("/public/v1/version", h.version)
	m.HandleFunc("/public/v1/sales", h.sales)
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
	oidc := auth.NewOIDCVerifier(c.OIDCIssuer, c.OIDCAudience, c.OIDCJWKSURL)
	return cors(c.CORSAllowedOrigins, auth.MiddlewareWithOIDC(c.AuthHMACKey, oidc, rateLimit(600, time.Minute, recoverer(version(c.APIVersion, enforceSuccessResponses(m))))))
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
func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	write(w, 200, map[string]string{"api": h.cfg.APIVersion, "build": "mvp-dev", "policy": "BG-2026-EUR", "schema": "1"})
}
func (h *Handler) sales(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) sale(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/sales/"), "/")
	parts := strings.Split(p, "/")
	id := parts[0]
	if !h.authorizeSale(w, r, id) {
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
	case "payments":
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
		if e = h.svc.QueueFiscalEvent(id, v); e != nil {
			problem(w, 500, "OUTBOX_PERSIST_FAILED")
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
		if e = h.svc.QueueFiscalEvent(id, v); e != nil {
			problem(w, 500, "OUTBOX_PERSIST_FAILED")
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
		page, err := paginate(r, h.svc.Shifts(tenantID(r)))
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
			problem(w, http.StatusNotFound, "EXPORT_ARTIFACT_NOT_FOUND")
			return
		}
		w.Header().Set("Content-Type", media)
		w.Header().Set("Content-Disposition", `attachment; filename="compliance-export-`+parts[2]+`"`)
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
	v, e := h.svc.GetOperationForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "OPERATION_NOT_FOUND")
		return
	}
	if len(p) == 1 && r.Method == "GET" {
		write(w, 200, v)
		return
	}
	if len(p) == 2 && p[1] == "reconcile" && r.Method == "POST" {
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
	k := replayKey(r)
	v, ok := h.svc.Replay(k)
	if !ok {
		return false
	}
	if v.Hash != domain.Hash(b) {
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
	if e := h.svc.PutReplay(k, domain.ReplayRecord{Hash: domain.Hash(b), Status: status, Body: out}); e != nil {
		problem(w, 500, "IDEMPOTENCY_PERSIST_FAILED")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(out)
}
func (h *Handler) saveEmptyReplay(w http.ResponseWriter, r *http.Request, b []byte, status int) {
	k := replayKey(r)
	if e := h.svc.PutReplay(k, domain.ReplayRecord{Hash: domain.Hash(b), Status: status, Body: []byte{}}); e != nil {
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
