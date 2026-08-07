package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"github.com/fxamacker/cbor/v2"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func testHandler() http.Handler {
	s := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	s.SetBLESigningKey("01234567890123456789012345678901")
	return NewHandler(s, config.Config{APIVersion: "2026-08-07", AllowStubAdapters: true})
}

func TestCountryPolicyAndEffectiveTaxGroups(t *testing.T) {
	h := testHandler()
	call := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Api-Version", "2026-08-07")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := call("/public/v1/country-policy")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"official_currency":"EUR"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"version":"bg-2026.08.07"`)) {
		t.Fatalf("country policy: %d %s", w.Code, w.Body.String())
	}
	w = call("/public/v1/tax-groups?effective_at=2026-08-07T00:00:00Z")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"code":"B"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"rate":"20.00"`)) {
		t.Fatalf("tax groups: %d %s", w.Code, w.Body.String())
	}
	w = call("/public/v1/tax-groups?effective_at=not-a-date")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid effective_at: %d %s", w.Code, w.Body.String())
	}
	w = call("/public/v1/tax-groups?effective_at=2025-12-31T23:59:59Z")
	if w.Code != http.StatusNotFound {
		t.Fatalf("pre-policy date: %d %s", w.Code, w.Body.String())
	}
}

func TestWebhookEndpointPublicLifecycle(t *testing.T) {
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), cfg)
	call := func(method, path, tenant, key, match string, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+jwt(tenant, "ADMIN"))
		r.Header.Set("X-Api-Version", "2026-08-07")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if match != "" {
			r.Header.Set("If-Match", match)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := call(http.MethodPost, "/public/v1/webhook-endpoints", "tenant-a", "webhook-create-0001", "", `{"url":"https://hooks.example.test/fiscal","events":["fiscal.operation.updated"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"].(string)
	if created["secret"] == "" {
		t.Fatal("create did not return one-time secret")
	}
	w = call(http.MethodGet, "/public/v1/webhook-endpoints/"+id, "tenant-a", "", "", "")
	if w.Code != http.StatusOK || bytes.Contains(w.Body.Bytes(), []byte(`"secret"`)) {
		t.Fatalf("GET status/leak: %d %s", w.Code, w.Body.String())
	}
	w = call(http.MethodGet, "/public/v1/webhook-endpoints/"+id, "tenant-b", "", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross tenant: %d %s", w.Code, w.Body.String())
	}
	w = call(http.MethodPost, "/public/v1/webhook-endpoints/"+id+"/rotate-secret", "tenant-a", "webhook-rotate-0001", "", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"previous_valid_until"`)) {
		t.Fatalf("rotate: %d %s", w.Code, w.Body.String())
	}
	w = call(http.MethodDelete, "/public/v1/webhook-endpoints/"+id, "tenant-a", "webhook-delete-0001", "", "")
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", w.Code, w.Body.String())
	}
	w = call(http.MethodDelete, "/public/v1/webhook-endpoints/"+id, "tenant-a", "webhook-delete-0001", "", "")
	if w.Code != http.StatusNoContent || w.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("delete replay: %d %s", w.Code, w.Body.String())
	}
}

func TestOpaqueCursorPagination(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?limit=2", nil)
	page, err := paginate(r, []int{10, 20, 30})
	if err != nil || page["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("first page: %#v %v", page, err)
	}
	cursor := page["page"].(map[string]any)["next_cursor"].(string)
	r = httptest.NewRequest(http.MethodGet, "/items?limit=2&cursor="+cursor, nil)
	page, err = paginate(r, []int{10, 20, 30})
	items := page["items"].([]int)
	if err != nil || len(items) != 1 || items[0] != 30 || page["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("second page: %#v %v", page, err)
	}
	for _, query := range []string{"?limit=0", "?limit=201", "?cursor=bad"} {
		if _, err = paginate(httptest.NewRequest(http.MethodGet, "/items"+query, nil), []int{1}); err == nil {
			t.Fatalf("invalid pagination accepted: %s", query)
		}
	}
}

func TestEdgeSyncAcceptsCanonicalCBOR(t *testing.T) {
	h := testHandler()
	body, err := cbor.Marshal(apiSyncBatch())
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip domain.EdgeSyncBatch
	if err = cbor.Unmarshal(body, &roundTrip); err != nil || domain.EdgeBatchHash(roundTrip) != roundTrip.BatchSHA256 {
		t.Fatalf("CBOR contract round trip changed batch hash: %#v err=%v computed=%s", roundTrip, err, domain.EdgeBatchHash(roundTrip))
	}
	r := httptest.NewRequest(http.MethodPost, "/public/v1/edge-sync/batches", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/cbor")
	r.Header.Set("X-Api-Version", "2026-08-07")
	r.Header.Set("Idempotency-Key", "edge-cbor-sync-001")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"committed_event_hash"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"operation_results"`)) {
		t.Fatalf("CBOR sync: %d %s", w.Code, w.Body.String())
	}
}
func jwt(tenant string, selected ...string) string {
	role := "CASHIER"
	if len(selected) > 0 {
		role = selected[0]
	}
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload, _ := json.Marshal(map[string]any{"sub": "user", "tenant_id": tenant, "roles": []string{role}, "scope": "fiscal.base", "exp": time.Now().Add(time.Hour).Unix()})
	body := base64.RawURLEncoding.EncodeToString(payload)
	m := hmac.New(sha256.New, []byte("01234567890123456789012345678901"))
	m.Write([]byte(head + "." + body))
	return head + "." + body + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func apiSyncBatch() domain.EdgeSyncBatch {
	e := domain.DeviceEventEnvelope{EventID: "event-1", OperationID: "operation-1", DeviceID: "device-1", JournalSeq: 1, EventType: "FISCALIZED", OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Payload: map[string]any{"state": "FISCALIZED"}}
	e.EventHash = domain.DeviceEventHash(e)
	v := domain.EdgeSyncBatch{EdgeID: "edge-1", SchemaVersion: "2026-08-07", FirstSeq: 1, LastSeq: 1, Events: []domain.DeviceEventEnvelope{e}}
	v.BatchSHA256 = domain.EdgeBatchHash(v)
	m := hmac.New(sha256.New, []byte("01234567890123456789012345678901"))
	m.Write([]byte(v.BatchSHA256))
	v.Signature = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	return v
}
func TestCrossTenantSaleIsHidden(t *testing.T) {
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), cfg)
	body := bytes.NewBufferString(`{"external_id":"e1","register_id":"r1","operator_id":"A001"}`)
	r := httptest.NewRequest("POST", "/public/v1/sales", body)
	r.Header.Set("Authorization", "Bearer "+jwt("tenant-a"))
	r.Header.Set("X-Api-Version", "2026-08-07")
	r.Header.Set("Idempotency-Key", "tenant-sale-key-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var sale map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &sale)
	r = httptest.NewRequest("GET", "/public/v1/sales/"+sale["sale_id"].(string), nil)
	r.Header.Set("Authorization", "Bearer "+jwt("tenant-b"))
	r.Header.Set("X-Api-Version", "2026-08-07")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("cross tenant status=%d %s", w.Code, w.Body.String())
	}
}

func TestAdministrativeSurfaceTenantIsolationAndBinding(t *testing.T) {
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), cfg)
	call := func(method, path, tenant, key string, body any, ifMatch string) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+jwt(tenant, "ADMIN"))
		r.Header.Set("X-Api-Version", "2026-08-07")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if ifMatch != "" {
			r.Header.Set("If-Match", ifMatch)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := call("PATCH", "/public/v1/organizations", "tenant-a", "org-key-123456789", map[string]any{"legal_name": "Bee Ltd", "eik": "123456789", "country": "BG", "status": "ACTIVE"}, "1")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", "/public/v1/locations", "tenant-a", "loc-key-123456789", map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"}, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var location map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &location)
	w = call("POST", "/public/v1/registers", "tenant-a", "reg-key-123456789", map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE"}, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var register map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &register)
	w = call("POST", "/public/v1/devices", "tenant-a", "dev-key-123456789", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "SN1", "status": "DRAFT", "environment": "DEV", "simulated": true}, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var device map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &device)
	w = call("POST", "/public/v1/registers/"+register["id"].(string)+"/bindings", "tenant-a", "bind-key-12345678", map[string]any{"device_id": device["id"], "role": "FISCAL_DEVICE"}, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("GET", "/public/v1/devices/"+device["id"].(string), "tenant-b", "", nil, "")
	if w.Code != 404 {
		t.Fatalf("cross tenant device status=%d %s", w.Code, w.Body.String())
	}
	w = call("GET", "/public/v1/devices/"+device["id"].(string)+"/capabilities", "tenant-a", "", nil, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}

type apiStore struct {
	mu sync.Mutex
	b  []byte
}

func (s *apiStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b...), nil
}
func (s *apiStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b = append([]byte(nil), b...)
	return nil
}
func TestIdempotencySurvivesRepositoryRestart(t *testing.T) {
	store := &apiStore{}
	r, e := domain.NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{APIVersion: "2026-08-07"}
	h := NewHandler(domain.NewService(r, domain.NewSimulator(true)), cfg)
	w := req(t, h, "POST", "/public/v1/sales", "restart-key-12345", map[string]any{"external_id": "e", "register_id": "r", "operator_id": "A001"})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	first := w.Body.String()
	r, e = domain.NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	h = NewHandler(domain.NewService(r, domain.NewSimulator(true)), cfg)
	w = req(t, h, "POST", "/public/v1/sales", "restart-key-12345", map[string]any{"external_id": "e", "register_id": "r", "operator_id": "A001"})
	if w.Code != 201 || w.Header().Get("Idempotency-Replayed") != "true" || w.Body.String() != first {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
func req(t *testing.T, h http.Handler, m, p, k string, v any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(v)
	r := httptest.NewRequest(m, p, bytes.NewReader(b))
	r.Header.Set("X-Api-Version", "2026-08-07")
	if k != "" {
		r.Header.Set("Idempotency-Key", k)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestSaleHappyPathAndIdempotency(t *testing.T) {
	h := testHandler()
	w := req(t, h, "POST", "/public/v1/sales", "1234567890123456", map[string]any{"external_id": "o1", "register_id": "FD000001", "operator_id": "A001"})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	id := s["sale_id"].(string)
	w = req(t, h, "POST", "/public/v1/sales/"+id+"/lines", "2234567890123456", map[string]any{"line_id": "l1", "name": "Coffee", "quantity": "1.000", "unit_price": map[string]string{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req(t, h, "POST", "/public/v1/sales/"+id+"/payments", "3234567890123456", map[string]any{"payment_id": "p1", "type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	w2 := req(t, h, "POST", "/public/v1/sales/"+id+"/payments", "3234567890123456", map[string]any{"payment_id": "p1", "type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}})
	if w2.Code != 202 || w2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal(w2.Code, w2.Header())
	}
}
func TestRejectsMissingVersionAndBGN(t *testing.T) {
	h := testHandler()
	r := httptest.NewRequest("POST", "/public/v1/sales", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = req(t, h, "POST", "/public/v1/sales", "4234567890123456", map[string]any{"external_id": "o2", "register_id": "FD000001", "operator_id": "A001"})
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	id := s["sale_id"].(string)
	w = req(t, h, "POST", "/public/v1/sales/"+id+"/lines", "5234567890123456", map[string]any{"line_id": "l1", "name": "Bad", "quantity": "1.000", "unit_price": map[string]string{"amount": "2.50", "currency": "BGN"}, "tax_group": "B"})
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestShiftReportsSyncAndDeviceBlockRules(t *testing.T) {
	h := testHandler()
	sh := req(t, h, "POST", "/public/v1/shifts", "6234567890123456", map[string]any{"register_id": "FD000001", "operator_id": "A001"})
	if sh.Code != 201 {
		t.Fatal(sh.Code, sh.Body.String())
	}
	report := req(t, h, "POST", "/public/v1/registers/FD000001/reports", "7234567890123456", map[string]any{"type": "Z"})
	if report.Code != 202 {
		t.Fatal(report.Code, report.Body.String())
	}
	sync := req(t, h, "POST", "/public/v1/edge-sync/batches", "8234567890123456", apiSyncBatch())
	if sync.Code != 200 {
		t.Fatal(sync.Code, sync.Body.String())
	}
	ready := req(t, h, "GET", "/public/v1/devices/device-1/readiness", "", nil)
	if ready.Code != 200 {
		t.Fatal(ready.Code, ready.Body.String())
	}
}

func TestCardTerminalUnavailableDoesNotFallbackToCash(t *testing.T) {
	h := testHandler()
	w := req(t, h, "POST", "/public/v1/sales", "9234567890123456", map[string]any{"external_id": "card-1", "register_id": "FD000001", "operator_id": "A001"})
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	id := s["sale_id"].(string)
	w = req(t, h, "POST", "/public/v1/sales/"+id+"/lines", "a234567890123456", map[string]any{"line_id": "l1", "name": "Coffee", "quantity": "1.000", "unit_price": map[string]string{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"})
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	w = req(t, h, "POST", "/public/v1/sales/"+id+"/payments", "b234567890123456", map[string]any{"payment_id": "p1", "type": "CARD", "terminal_policy": "REQUIRED", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var op map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &op)
	if op["state"] != "FAILED" || op["error_code"] != "PAYMENT_TERMINAL_UNAVAILABLE" {
		t.Fatal(op)
	}
}
