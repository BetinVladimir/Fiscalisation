package localapi

import (
	"crypto/hmac"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"fiscalisation/edge-agent/gateway"
	edgeruntime "fiscalisation/edge-agent/runtime"
)

type Binding struct {
	TenantID, RegisterID, DeviceID string
	FencingToken                   int64
}

// Handler is a loopback-only executable adapter used for DEV smoke/HIL bridge
// processes. Production commands use the authenticated BLE GATT processor.
type Handler struct {
	runtime *edgeruntime.Runtime
	gateway *gateway.ComplianceGateway
	binding Binding
	token   string
}

func New(runtime *edgeruntime.Runtime, binding Binding, token string) *Handler {
	return &Handler{runtime: runtime, binding: binding, token: token}
}

// NewWithCompliance exposes the loopback compliance-intent surface used by a
// co-located POS. The bearer is session/app authority, never a public static
// deployment secret. Raw device commands remain DEV/HIL-only.
func NewWithCompliance(runtime *edgeruntime.Runtime, compliance *gateway.ComplianceGateway, binding Binding, token string) *Handler {
	return &Handler{runtime: runtime, gateway: compliance, binding: binding, token: token}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	enforceEdgeSuccess(w, r, h.serveHTTP)
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil || len(h.token) < 16 || !h.authorized(r) {
		edgeProblem(w, http.StatusUnauthorized, "EDGE_UNAUTHORIZED", "unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/storage":
		status, err := h.runtime.StorageStatus()
		if err != nil {
			edgeProblem(w, http.StatusServiceUnavailable, "EDGE_STORAGE_UNAVAILABLE", "storage status unavailable")
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/final-device":
		if err := h.runtime.ProbeFinalDevice(); err != nil {
			edgeProblem(w, http.StatusServiceUnavailable, "FISCAL_DEVICE_UNREACHABLE", "final fiscal device is unreachable")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"state": "READY"})
	case r.Method == http.MethodPost && r.URL.Path == "/internal/v1/commands":
		var command edgeruntime.Command
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&command) != nil || !h.matches(command) {
			edgeProblem(w, http.StatusUnprocessableEntity, "EDGE_COMMAND_INVALID", "invalid command or binding")
			return
		}
		result, err := h.runtime.Execute(command)
		if err != nil {
			edgeProblem(w, http.StatusConflict, "EDGE_EXECUTION_REJECTED", "edge execution rejected")
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	case r.Method == http.MethodPost && r.URL.Path == "/local/v1/intents":
		if h.gateway == nil {
			edgeProblem(w, http.StatusServiceUnavailable, "COMPLIANCE_GATEWAY_UNAVAILABLE", "local compliance gateway unavailable")
			return
		}
		var intent gateway.ComplianceIntent
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&intent) != nil {
			edgeProblem(w, http.StatusUnprocessableEntity, "COMPLIANCE_INTENT_INVALID", "invalid compliance intent")
			return
		}
		result, err := h.gateway.Execute(intent)
		if err != nil {
			edgeProblem(w, http.StatusConflict, "COMPLIANCE_INTENT_REJECTED", "compliance intent rejected")
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	default:
		edgeProblem(w, http.StatusNotFound, "NOT_FOUND", "route not found")
	}
}

func edgeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "urn:beefiscal-edge:error:" + strings.ToLower(code), "title": code, "status": status, "code": code, "retryable": status == http.StatusServiceUnavailable, "detail": detail, "trace_id": time.Now().UTC().Format("20060102150405.000000000")})
}

func (h *Handler) authorized(r *http.Request) bool {
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return hmac.Equal([]byte(provided), []byte(h.token))
}

func (h *Handler) matches(v edgeruntime.Command) bool {
	return v.TenantID == h.binding.TenantID && v.RegisterID == h.binding.RegisterID && v.DeviceID == h.binding.DeviceID && v.FencingToken == h.binding.FencingToken
}
