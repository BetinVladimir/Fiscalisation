package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func do(h http.Handler, m, p, k string, v any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(v)
	r := httptest.NewRequest(m, p, bytes.NewReader(b))
	if k != "" {
		r.Header.Set("Idempotency-Key", k)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
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
func TestPublicIdempotencySurvivesRestartAndRejectsMismatch(t *testing.T) {
	store := &apiStore{}
	svc, err := domain.NewPersistentService("http://invalid", "2026-08-07", store)
	if err != nil {
		t.Fatal(err)
	}
	h := New(svc, config.Config{APIVersion: "2026-08-07"})
	doReq := func(handler http.Handler, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/public/v1/minipos/products", bytes.NewBufferString(body))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Idempotency-Key", "persistent-key-01")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := `{"sku":"C1","name":"Coffee","price":{"amount":"2.50","currency":"EUR"},"tax_group":"B"}`
	first := doReq(h, body)
	if first.Code != 201 {
		t.Fatal(first.Code, first.Body.String())
	}
	svc, err = domain.NewPersistentService("http://invalid", "2026-08-07", store)
	if err != nil {
		t.Fatal(err)
	}
	h = New(svc, config.Config{APIVersion: "2026-08-07"})
	again := doReq(h, body)
	if again.Code != 201 || again.Header().Get("Idempotency-Replayed") != "true" || again.Body.String() != first.Body.String() {
		t.Fatal(again.Code, again.Header(), again.Body.String())
	}
	mismatch := doReq(h, `{"sku":"C2","name":"Other","price":{"amount":"3.00","currency":"EUR"},"tax_group":"B"}`)
	if mismatch.Code != 409 {
		t.Fatal(mismatch.Code, mismatch.Body.String())
	}
}
func TestEditorsShiftOrder(t *testing.T) {
	h := New(domain.NewService("http://invalid", "2026-08-07"), config.Config{})
	e := do(h, "POST", "/api/v1/employees", "", map[string]any{"first_name": "Ada", "last_name": "Lovelace", "operator_code": "A001"})
	if e.Code != 201 {
		t.Fatal(e.Code, e.Body.String())
	}
	var emp map[string]any
	_ = json.Unmarshal(e.Body.Bytes(), &emp)
	sh := do(h, "POST", "/api/v1/shifts", "", map[string]any{"register_id": "FD000001", "employee_id": emp["id"]})
	if sh.Code != 201 {
		t.Fatal(sh.Code, sh.Body.String())
	}
	var shift map[string]any
	_ = json.Unmarshal(sh.Body.Bytes(), &shift)
	o := do(h, "POST", "/api/v1/orders", "", map[string]any{"shift_id": shift["id"]})
	if o.Code != 201 {
		t.Fatal(o.Code, o.Body.String())
	}
}
func TestWebhookRejectsInvalidSignature(t *testing.T) {
	h := New(domain.NewService("http://invalid", "2026-08-07"), config.Config{WebhookVerificationKey: "secret"})
	w := do(h, "POST", "/api/v1/fiscal-webhooks", "", map[string]any{"aggregate_id": "sale"})
	if w.Code != 401 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCanonicalWebhookSignatureAndTimestamp(t *testing.T) {
	body := []byte(`{"event_id":"evt-1","event_type":"fiscal.operation.updated","api_version":"2026-08-07","tenant_id":"tenant-a","resource_id":"sale-1","resource_version":1,"data":{"state":"FISCALIZED","operation_id":"op-1"}}`)
	sign := func(at time.Time) string {
		ts := strconv.FormatInt(at.Unix(), 10)
		m := hmac.New(sha256.New, []byte("secret"))
		m.Write([]byte(ts + "."))
		m.Write(body)
		return "t=" + ts + ",kid=active,v1=" + hex.EncodeToString(m.Sum(nil))
	}
	h := New(domain.NewService("http://invalid", "2026-08-07"), config.Config{APIVersion: "2026-08-07", WebhookVerificationKey: "secret"})
	request := func(signature string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/fiscal-webhooks", bytes.NewReader(body))
		r.Header.Set("BeeFiscal-Signature", signature)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request(sign(time.Now().UTC())); w.Code == http.StatusUnauthorized {
		t.Fatalf("valid signature rejected: %d %s", w.Code, w.Body.String())
	}
	if w := request(sign(time.Now().UTC().Add(-10 * time.Minute))); w.Code != http.StatusUnauthorized {
		t.Fatalf("stale signature accepted: %d", w.Code)
	}
	tampered := append([]byte(nil), body...)
	tampered[len(tampered)-2] = 'X'
	if validWebhookSignature(sign(time.Now().UTC()), tampered, []byte("secret"), time.Now().UTC()) {
		t.Fatal("tampered payload accepted")
	}
}

func TestPublicContractAliasAndOptimisticUpdate(t *testing.T) {
	h := New(domain.NewService("http://invalid", "2026-08-07"), config.Config{APIVersion: "2026-08-07"})
	r := httptest.NewRequest("POST", "/public/v1/minipos/products", bytes.NewBufferString(`{"sku":"C1","name":"Coffee","price":{"amount":"2.50","currency":"EUR"},"tax_group":"B"}`))
	r.Header.Set("X-Api-Version", "2026-08-07")
	r.Header.Set("Idempotency-Key", "product-create-key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p domain.Product
	if json.Unmarshal(w.Body.Bytes(), &p) != nil {
		t.Fatal("decode")
	}
	r = httptest.NewRequest("PATCH", "/public/v1/minipos/products/"+p.ID, bytes.NewBufferString(`{"sku":"C1","name":"Coffee XL","price":{"amount":"3.00","currency":"EUR"},"tax_group":"B","active":true}`))
	r.Header.Set("X-Api-Version", "2026-08-07")
	r.Header.Set("If-Match", "1")
	r.Header.Set("Idempotency-Key", "product-update-key")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "/public/v1/minipos/products", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("version must be mandatory")
	}
}

func TestAutonomousConfigurationCreateUpdateAndVersionConflict(t *testing.T) {
	h := New(domain.NewService("http://invalid", "2026-08-07"), config.Config{APIVersion: "2026-08-07"})
	body := `{"location_name":"Sofia Shop","location_address":"1 Main St","workstation_name":"POS 01","fiscal_register_id":"FD000001"}`
	request := func(key, version, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPatch, "/public/v1/minipos/configuration", bytes.NewBufferString(payload))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Idempotency-Key", key)
		if version != "" {
			r.Header.Set("If-Match", version)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	first := request("configuration-create", "", body)
	if first.Code != http.StatusOK {
		t.Fatal(first.Code, first.Body.String())
	}
	var created domain.Configuration
	if json.Unmarshal(first.Body.Bytes(), &created) != nil || created.Version != 1 {
		t.Fatal("invalid created configuration")
	}
	updatedBody := `{"location_name":"Sofia Shop","location_address":"2 Main St","workstation_name":"POS 01","fiscal_register_id":"FD000001"}`
	updated := request("configuration-update", "1", updatedBody)
	if updated.Code != http.StatusOK {
		t.Fatal(updated.Code, updated.Body.String())
	}
	conflict := request("configuration-conflict", "1", body)
	if conflict.Code != http.StatusConflict {
		t.Fatal("stale update accepted", conflict.Code)
	}
	r := httptest.NewRequest(http.MethodGet, "/public/v1/minipos/configuration", nil)
	r.Header.Set("X-Api-Version", "2026-08-07")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "2 Main St") {
		t.Fatal(w.Code, w.Body.String())
	}
}
