package localapi

import (
	"crypto/hmac"
	"encoding/json"
	"net/http"
	"strings"

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
	binding Binding
	token   string
}

func New(runtime *edgeruntime.Runtime, binding Binding, token string) *Handler {
	return &Handler{runtime: runtime, binding: binding, token: token}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil || len(h.token) < 16 || !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/storage":
		status, err := h.runtime.StorageStatus()
		if err != nil {
			http.Error(w, "storage status unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/final-device":
		if err := h.runtime.ProbeFinalDevice(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"state": "BLOCK", "error_code": "FISCAL_DEVICE_UNREACHABLE"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"state": "READY"})
	case r.Method == http.MethodPost && r.URL.Path == "/internal/v1/commands":
		var command edgeruntime.Command
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&command) != nil || !h.matches(command) {
			http.Error(w, "invalid command or binding", http.StatusUnprocessableEntity)
			return
		}
		result, err := h.runtime.Execute(command)
		if err != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"state": "REJECTED", "error_code": "EDGE_EXECUTION_REJECTED"})
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) authorized(r *http.Request) bool {
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return hmac.Equal([]byte(provided), []byte(h.token))
}

func (h *Handler) matches(v edgeruntime.Command) bool {
	return v.TenantID == h.binding.TenantID && v.RegisterID == h.binding.RegisterID && v.DeviceID == h.binding.DeviceID && v.FencingToken == h.binding.FencingToken
}
