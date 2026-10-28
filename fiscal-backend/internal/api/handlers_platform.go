package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"fiscalisation/fiscal-backend/internal/domain"
	"github.com/fxamacker/cbor/v2"
)

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

func (h *Handler) live(w http.ResponseWriter, _ *http.Request) {
	write(w, 200, map[string]any{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()
	components := h.svc.Ready(ctx)

	anyUnhealthy := false
	for i := range components {
		if components[i].Status == "unhealthy" {
			anyUnhealthy = true
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if anyUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := enc.Encode(struct {
			Status string                  `json:"status"`
			Items  []domain.ReadyComponent `json:"items"`
		}{"unhealthy", components}); err != nil {
			log.Printf("readyz: encode failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if err := enc.Encode(struct {
		Status string                  `json:"status"`
		Items  []domain.ReadyComponent `json:"items"`
	}{"ok", components}); err != nil {
		log.Printf("readyz: encode failed: %v", err)
	}
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

