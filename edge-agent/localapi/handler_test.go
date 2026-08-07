package localapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"fiscalisation/edge-agent/authority"
	"fiscalisation/edge-agent/device"
	"fiscalisation/edge-agent/journal"
	edgeruntime "fiscalisation/edge-agent/runtime"
)

func TestExecutableCommandIsDurableIdempotentAndBound(t *testing.T) {
	j, err := journal.Open(filepath.Join(t.TempDir(), "edge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	d := device.NewSimulator(true)
	a := authority.New(authority.Lease{RegisterID: "register", EdgeID: "edge", FencingToken: 7, OperationFrom: 1, OperationTo: 10, UNPFrom: 11, UNPTo: 20, ExpiresAt: time.Now().Add(time.Hour)})
	runtime := edgeruntime.New(j, a, d)
	h := New(runtime, Binding{TenantID: "tenant", RegisterID: "register", DeviceID: "device", FencingToken: 7}, "1234567890123456")
	command := edgeruntime.Command{CommandID: "operation", TenantID: "tenant", RegisterID: "register", DeviceID: "device", Type: "FISCAL_SALE", FencingToken: 7, Payload: json.RawMessage(`{"metadata":{}}`)}

	call := func(v edgeruntime.Command) *httptest.ResponseRecorder {
		body, _ := json.Marshal(v)
		r := httptest.NewRequest(http.MethodPost, "/internal/v1/commands", bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer 1234567890123456")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call(command); w.Code != http.StatusOK {
		t.Fatalf("command failed: %d %s", w.Code, w.Body.String())
	}
	if w := call(command); w.Code != http.StatusOK || d.Executions("operation") != 1 {
		t.Fatalf("idempotent replay executed twice: %d", d.Executions("operation"))
	}
	bad := command
	bad.TenantID = "other"
	if w := call(bad); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cross-tenant command accepted: %d", w.Code)
	}
	if len(j.Events()) != 2 {
		t.Fatalf("expected durable command/result, got %d", len(j.Events()))
	}
}

func TestProbeAndAuthenticationFailClosed(t *testing.T) {
	j, _ := journal.Open(filepath.Join(t.TempDir(), "edge.db"))
	defer j.Close()
	d := device.NewSimulator(false)
	runtime := edgeruntime.New(j, authority.New(authority.Lease{FencingToken: 1, OperationFrom: 1, OperationTo: 1, UNPFrom: 1, UNPTo: 1, ExpiresAt: time.Now().Add(time.Hour)}), d)
	h := New(runtime, Binding{}, "1234567890123456")
	r := httptest.NewRequest(http.MethodGet, "/internal/v1/final-device", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatal("missing token accepted")
	}
	r.Header.Set("Authorization", "Bearer 1234567890123456")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unreachable device reported ready: %d", w.Code)
	}
}

func TestStorageStatusIsAuthenticatedAndExposesThresholdState(t *testing.T) {
	j, _ := journal.Open(filepath.Join(t.TempDir(), "edge.db"))
	defer j.Close()
	runtime := edgeruntime.New(j, authority.New(authority.Lease{}), device.NewSimulator(true))
	runtime.SetStorageQuota(1)
	h := New(runtime, Binding{}, "1234567890123456")
	r := httptest.NewRequest(http.MethodGet, "/internal/v1/storage", nil)
	r.Header.Set("Authorization", "Bearer 1234567890123456")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"state":"FULL"`)) {
		t.Fatalf("unexpected storage response: %d %s", w.Code, w.Body.String())
	}
}
