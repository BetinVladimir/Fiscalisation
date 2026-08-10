package localapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fiscalisation/edge-agent/authority"
	"fiscalisation/edge-agent/device"
	"fiscalisation/edge-agent/gateway"
	"fiscalisation/edge-agent/journal"
	edgeruntime "fiscalisation/edge-agent/runtime"
)

func TestLocalComplianceIntentIsAuthenticatedDurableAndOpaque(t *testing.T) {
	j, err := journal.Open(filepath.Join(t.TempDir(), "edge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	now := time.Now().UTC()
	d := device.NewSimulator(true)
	runtime := edgeruntime.New(j, authority.New(authority.Lease{RegisterID: "register", EdgeID: "edge", FencingToken: 7, OperationFrom: 1, OperationTo: 10, UNPFrom: 41, UNPTo: 50, ExpiresAt: now.Add(time.Hour)}), d)
	session := gateway.SessionBinding{TenantID: "tenant", RegisterID: "register", DeviceID: "device", SessionID: "session", OperatorCode: "A001", AppInstanceID: "00000000-0000-4000-8000-000000000013", FencingToken: 7, ExpiresAt: now.Add(time.Hour), IsRevoked: func(string, time.Time) bool { return false }}
	compliance, err := gateway.NewComplianceGateway(runtime, session, gateway.CountryPolicyBundle{CountryCode: "BG", ProfileVersion: "2026.1", IdentifierScheme: "BG_UNP_V1", FiscalDeviceNumber: "AB123456", Signature: "trusted-signature", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	h := NewWithCompliance(runtime, compliance, Binding{TenantID: "tenant", RegisterID: "register", DeviceID: "device", FencingToken: 7}, "1234567890123456")
	body := `{"intent_id":"00000000-0000-4000-8000-000000000011","action":"OPEN_WITH_LINE","client_sale_surrogate_id":"00000000-0000-4000-8000-000000000012","operator_code":"A001","app_instance_id":"00000000-0000-4000-8000-000000000013","expected_version":0,"line":{"line_id":"00000000-0000-4000-8000-000000000014","name":"Coffee","quantity":"1.000","unit_price":"2.50","tax_group":"B"}}`
	call := func(token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/local/v1/intents", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated intent accepted: %d", w.Code)
	}
	first := call("1234567890123456")
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), "AB123456-A001-0000041") {
		r := httptest.NewRequest(http.MethodPost, "/local/v1/intents", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer 1234567890123456")
		raw := httptest.NewRecorder()
		h.serveHTTP(raw, r)
		var decoded any
		_ = json.Unmarshal(raw.Body.Bytes(), &decoded)
		var schemaErr error
		for _, contract := range generatedSuccessResponses {
			if contract.Path == "/local/v1/intents" {
				schemaErr = validateResponseSchema(contract.Schema, decoded)
			}
		}
		t.Fatalf("local intent failed: %d %s raw=%d %s schema=%v", first.Code, first.Body.String(), raw.Code, raw.Body.String(), schemaErr)
	}
	if replay := call("1234567890123456"); replay.Code != http.StatusOK || d.Executions("00000000-0000-4000-8000-000000000011") != 1 {
		t.Fatalf("local replay not idempotent: %d", replay.Code)
	}
}

func TestEdgeGeneratedSuccessResponseMiddlewareFailsClosed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/internal/v1/storage", nil)
	w := httptest.NewRecorder()
	enforceEdgeSuccess(w, r, func(inner http.ResponseWriter, _ *http.Request) {
		inner.Header().Set("Content-Type", "text/plain")
		inner.WriteHeader(http.StatusOK)
		_, _ = inner.Write([]byte(`{}`))
	})
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "response contract violation") {
		t.Fatalf("undocumented Edge media accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestEdgeProblemResponseFailsClosed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/internal/v1/storage", nil)
	w := httptest.NewRecorder()
	enforceEdgeSuccess(w, r, func(inner http.ResponseWriter, _ *http.Request) {
		inner.Header().Set("Content-Type", "application/problem+json")
		inner.WriteHeader(http.StatusConflict)
		_, _ = inner.Write([]byte(`{"type":"urn:test","title":"bad","status":400,"code":"BAD","retryable":false,"trace_id":"trace"}`))
	})
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "response contract violation") {
		t.Fatalf("invalid Problem accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestEdgeRequestContractRejectsUnknownField(t *testing.T) {
	called := false
	r := httptest.NewRequest(http.MethodPost, "/internal/v1/commands", bytes.NewBufferString(`{"command_id":"c","tenant_id":"t","register_id":"r","device_id":"d","type":"FISCAL_SALE","fencing_token":1,"payload":{},"unexpected":true}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	enforceEdgeSuccess(w, r, func(http.ResponseWriter, *http.Request) { called = true })
	if w.Code != http.StatusUnprocessableEntity || called || !strings.Contains(w.Body.String(), "REQUEST_CONTRACT_VIOLATION") {
		t.Fatalf("invalid Edge request reached runtime: %d called=%v %s", w.Code, called, w.Body.String())
	}
}

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
		r.Header.Set("Content-Type", "application/json")
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
