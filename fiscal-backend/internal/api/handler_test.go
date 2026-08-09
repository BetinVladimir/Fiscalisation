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
	"strings"
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
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
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

func TestWebhookCreatedResponseIsClosedAndFailClosed(t *testing.T) {
	now := time.Now().UTC()
	valid := map[string]any{
		"id": "123e4567-e89b-42d3-a456-426614174000", "version": int64(1),
		"url": "https://hooks.example.test/fiscal", "events": []any{"fiscal.operation.updated"},
		"status": "ACTIVE", "created_at": now, "updated_at": now,
		"secret": base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)),
	}
	response, err := typedWebhookCreated(valid)
	if err != nil || response.ID != valid["id"] || len(response.Events) != 1 {
		t.Fatalf("valid response rejected: %+v %v", response, err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"extra":  func(v map[string]any) { v["tenant_id"] = "leak" },
		"secret": func(v map[string]any) { v["secret"] = "short" },
		"status": func(v map[string]any) { v["status"] = "DISABLED" },
		"events": func(v map[string]any) { v["events"] = []any{"same", "same"} },
	} {
		copy := make(map[string]any, len(valid))
		for key, value := range valid {
			copy[key] = value
		}
		mutate(copy)
		if _, err = typedWebhookCreated(copy); err == nil {
			t.Fatalf("%s response contract violation accepted", name)
		}
	}
}

func TestGeneratedSuccessResponseMiddlewareFailsClosed(t *testing.T) {
	headers := make(http.Header)
	headers.Set("X-Api-Version", "2026-08-07")
	if err := validateGeneratedRequestParameters(`[{"name":"X-Api-Version","in":"header","required":true,"schema":{"type":"string","const":"2026-08-07"}}]`, "/version", "/version", nil, headers); err != nil {
		t.Fatalf("valid version parameter rejected: %v", err)
	}
	request := func(path string, status int, contentType, body string) *httptest.ResponseRecorder {
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		})
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Api-Version", "2026-08-07")
		w := httptest.NewRecorder()
		enforceSuccessResponses(next).ServeHTTP(w, r)
		return w
	}
	if w := request("/public/v1/version", http.StatusOK, "application/json", `{"build":"test","api":"2026-08-07","policy":"BG","schema":"v1"}`); w.Code != http.StatusOK {
		t.Fatalf("documented response rejected: %d %s", w.Code, w.Body.String())
	}
	for name, w := range map[string]*httptest.ResponseRecorder{
		"status":       request("/public/v1/version", http.StatusCreated, "application/json", `{}`),
		"media":        request("/public/v1/version", http.StatusOK, "text/plain", `{}`),
		"json":         request("/public/v1/version", http.StatusOK, "application/json", `{`),
		"undocumented": request("/public/v1/not-in-openapi", http.StatusOK, "application/json", `{}`),
	} {
		if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "RESPONSE_CONTRACT_VIOLATION") {
			t.Fatalf("%s drift accepted: %d %s", name, w.Code, w.Body.String())
		}
	}
}

func TestProblemResponseFailsClosedOnSchemaOrStatusMismatch(t *testing.T) {
	for name, body := range map[string]string{
		"missing required field": `{"type":"urn:test","title":"bad","status":409,"code":"BAD","retryable":false}`,
		"status mismatch":        `{"type":"urn:test","title":"bad","status":400,"code":"BAD","retryable":false,"trace_id":"trace"}`,
	} {
		t.Run(name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(body))
			})
			r := httptest.NewRequest(http.MethodGet, "/public/v1/version", nil)
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			enforceSuccessResponses(next).ServeHTTP(w, r)
			if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "RESPONSE_CONTRACT_VIOLATION") {
				t.Fatalf("invalid Problem accepted: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestRequestContractRejectsWrongFieldTypeBeforeHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	r := httptest.NewRequest(http.MethodPost, "/public/v1/sales", bytes.NewBufferString(`{"external_id":"e","register_id":"r","operator_id":1001}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	enforceSuccessResponses(next).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || called || !strings.Contains(w.Body.String(), "REQUEST_CONTRACT_VIOLATION") {
		t.Fatalf("invalid request reached handler: %d called=%v %s", w.Code, called, w.Body.String())
	}
}

func TestRequestContractRejectsInvalidQueryAndMissingIfMatch(t *testing.T) {
	for name, request := range map[string]*http.Request{
		"limit above canonical maximum":        httptest.NewRequest(http.MethodGet, "/public/v1/devices?limit=201", nil),
		"missing optimistic concurrency guard": httptest.NewRequest(http.MethodPost, "/public/v1/sales/sale-1/lines", bytes.NewBufferString(`{"line_id":"l1","name":"Coffee","quantity":"1.000","unit_price":{"amount":"2.50","currency":"EUR"},"tax_group":"B"}`)),
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			request.Header.Set("X-Api-Version", "2026-08-07")
			if request.Method == http.MethodPost {
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "parameter-test-1234")
			}
			w := httptest.NewRecorder()
			enforceSuccessResponses(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(w, request)
			if w.Code != http.StatusBadRequest || called {
				t.Fatalf("invalid parameter reached handler: %d called=%v %s", w.Code, called, w.Body.String())
			}
		})
	}
}

func TestCORSPreflightExposesEveryFiscalPublicMutationMethod(t *testing.T) {
	nextCalled := false
	handler := cors("https://admin.example, https://pos.example", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	request := httptest.NewRequest(http.MethodOptions, "/public/v1/webhook-endpoints/00000000-0000-4000-8000-000000000001", nil)
	request.Header.Set("Origin", "https://admin.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || nextCalled {
		t.Fatalf("valid preflight failed: status=%d called=%v body=%s", response.Code, nextCalled, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example" {
		t.Fatalf("unexpected allowed origin %q", got)
	}
	methods := response.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		if !strings.Contains(methods, method) {
			t.Fatalf("public method %s missing from preflight: %q", method, methods)
		}
	}

	denied := httptest.NewRequest(http.MethodOptions, "/public/v1/webhook-endpoints/00000000-0000-4000-8000-000000000001", nil)
	denied.Header.Set("Origin", "https://attacker.example")
	denied.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	deniedResponse := httptest.NewRecorder()
	handler.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden || deniedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign origin accepted: status=%d origin=%q", deniedResponse.Code, deniedResponse.Header().Get("Access-Control-Allow-Origin"))
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
	return jwtSubject(tenant, "user", selected...)
}
func jwtSubject(tenant, subject string, selected ...string) string {
	role := "CASHIER"
	if len(selected) > 0 {
		role = selected[0]
	}
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload, _ := json.Marshal(map[string]any{"sub": subject, "tenant_id": tenant, "roles": []string{role}, "scope": "fiscal.base", "exp": time.Now().Add(time.Hour).Unix()})
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
	svc := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	registerID := provisionAPIRegister(t, svc, "tenant-a")
	h := NewHandler(svc, cfg)
	body := bytes.NewBufferString(`{"external_id":"e1","register_id":"` + registerID + `","operator_id":"A001"}`)
	r := httptest.NewRequest("POST", "/public/v1/sales", body)
	r.Header.Set("Content-Type", "application/json")
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

func TestPublicBLESessionRejectsUnknownOperator(t *testing.T) {
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	svc := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	registerID := provisionAPIRegister(t, svc, "tenant-a")
	h := NewHandler(svc, cfg)
	call := func(operator, key string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"operator_id": operator, "app_instance_id": "00000000-0000-4000-8000-000000000001", "public_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
		r := httptest.NewRequest(http.MethodPost, "/public/v1/registers/"+registerID+"/ble-sessions", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+jwt("tenant-a"))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("NONE", "ble-unknown-op-0001"); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "BLE_SESSION_UNAVAILABLE") {
		t.Fatalf("unknown operator BLE authority response: %d %s", w.Code, w.Body.String())
	}
	if w := call("A001", "ble-active-op-00001"); w.Code != http.StatusCreated {
		t.Fatalf("active operator BLE authority rejected: %d %s", w.Code, w.Body.String())
	}
}

func TestPublicBLESessionLifecycleIsSubjectBoundAndRevokeIsCanonical204(t *testing.T) {
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	svc := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	registerID := provisionAPIRegister(t, svc, "tenant-a")
	h := NewHandler(svc, cfg)
	request := func(path, subject, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		r.Header.Set("Authorization", "Bearer "+jwtSubject("tenant-a", subject))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	created := request("/public/v1/registers/"+registerID+"/ble-sessions", "subject-1", "ble-subject-create-0001", `{"operator_id":"A001","app_instance_id":"00000000-0000-4000-8000-000000000001","public_key":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var session map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	id := session["ble_session_id"].(string)
	foreign := request("/public/v1/ble-sessions/"+id+"/refresh", "subject-2", "ble-subject-refresh-1", "")
	if foreign.Code != http.StatusConflict {
		t.Fatalf("foreign refresh: %d %s", foreign.Code, foreign.Body.String())
	}
	refreshed := request("/public/v1/ble-sessions/"+id+"/refresh", "subject-1", "ble-subject-refresh-2", "")
	if refreshed.Code != http.StatusOK {
		t.Fatalf("owner refresh: %d %s", refreshed.Code, refreshed.Body.String())
	}
	revoked := request("/public/v1/ble-sessions/"+id+"/revoke", "subject-1", "ble-subject-revoke-01", "")
	if revoked.Code != http.StatusNoContent || revoked.Body.Len() != 0 {
		t.Fatalf("revoke contract: %d %q", revoked.Code, revoked.Body.String())
	}
	replayed := request("/public/v1/ble-sessions/"+id+"/revoke", "subject-1", "ble-subject-revoke-01", "")
	if replayed.Code != http.StatusNoContent || replayed.Body.Len() != 0 || replayed.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("revoke replay contract: %d %q replay=%q", replayed.Code, replayed.Body.String(), replayed.Header().Get("Idempotency-Replayed"))
	}
}

func provisionAPIRegister(t *testing.T, svc *domain.Service, tenant string) string {
	t.Helper()
	location, err := svc.CreateResource("location", tenant, map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	register, err := svc.CreateResource("register", tenant, map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	deviceData := map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "API-FD-1", "status": "DRAFT", "environment": "DEV", "simulated": true}
	device, err := svc.CreateResource("device", tenant, deviceData)
	if err != nil {
		t.Fatal(err)
	}
	deviceData["status"] = "PENDING_SERVICE_ACTIVATION"
	device, err = svc.UpdateResource("device", device["id"].(string), tenant, device["version"].(int64), deviceData)
	if err != nil {
		t.Fatal(err)
	}
	deviceData["status"] = "ACTIVE"
	device, err = svc.UpdateResource("device", device["id"].(string), tenant, device["version"].(int64), deviceData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.BindRegister(register["id"].(string), tenant, device["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.CreateResource("operator", tenant, map[string]any{"code": "A001", "first_name": "Ada", "last_name": "Lovelace", "roles": []any{"CASHIER"}, "active_from": "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	return register["id"].(string)
}

func TestAdministrativeSurfaceTenantIsolationAndBinding(t *testing.T) {
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), cfg)
	call := func(method, path, tenant, key string, body any, ifMatch string) *httptest.ResponseRecorder {
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
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
	deviceBody := map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "SN1", "status": "DRAFT", "environment": "DEV", "simulated": true}
	w = call("POST", "/public/v1/devices", "tenant-a", "dev-key-123456789", deviceBody, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var device map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &device)
	deviceBody["status"] = "PENDING_SERVICE_ACTIVATION"
	w = call("PATCH", "/public/v1/devices/"+device["id"].(string), "tenant-a", "dev-pending-key-1234", deviceBody, "1")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &device)
	deviceBody["status"] = "ACTIVE"
	w = call("PATCH", "/public/v1/devices/"+device["id"].(string), "tenant-a", "dev-active-key-12345", deviceBody, "2")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &device)
	w = call("POST", "/public/v1/registers/"+register["id"].(string)+"/bindings", "tenant-a", "bind-key-12345678", map[string]any{"device_id": device["id"], "role": "FISCAL_DEVICE"}, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", "/public/v1/devices/"+device["id"].(string)+"/provisioning-sessions", "tenant-a", "provision-key-123456789", nil, "")
	if w.Code != 409 || !strings.Contains(w.Body.String(), "DEVICE_NOT_PROVISIONABLE") {
		t.Fatalf("active device provisioning status=%d %s", w.Code, w.Body.String())
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
	return reqWithHeaders(t, h, m, p, k, v, nil)
}
func reqWithHeaders(t *testing.T, h http.Handler, m, p, k string, v any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var b []byte
	if v != nil {
		b, _ = json.Marshal(v)
	}
	r := httptest.NewRequest(m, p, bytes.NewReader(b))
	r.Header.Set("X-Api-Version", "2026-08-07")
	if v != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if k != "" {
		r.Header.Set("Idempotency-Key", k)
	}
	for name, value := range headers {
		r.Header.Set(name, value)
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
	w = reqWithHeaders(t, h, "POST", "/public/v1/sales/"+id+"/lines", "2234567890123456", map[string]any{"line_id": "l1", "name": "Coffee", "quantity": "1.000", "unit_price": map[string]string{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"}, map[string]string{"If-Match": "1"})
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
	w = reqWithHeaders(t, h, "POST", "/public/v1/sales/"+id+"/lines", "5234567890123456", map[string]any{"line_id": "l1", "name": "Bad", "quantity": "1.000", "unit_price": map[string]string{"amount": "2.50", "currency": "BGN"}, "tax_group": "B"}, map[string]string{"If-Match": "1"})
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestShiftReportsSyncAndDeviceBlockRules(t *testing.T) {
	h := testHandler()
	registerID := "123e4567-e89b-42d3-a456-426614174001"
	operatorID := "123e4567-e89b-42d3-a456-426614174002"
	sh := req(t, h, "POST", "/public/v1/shifts", "6234567890123456", map[string]any{"register_id": registerID, "operator_id": operatorID})
	if sh.Code != 201 {
		t.Fatal(sh.Code, sh.Body.String())
	}
	report := req(t, h, "POST", "/public/v1/registers/"+registerID+"/reports", "7234567890123456", map[string]any{"type": "Z"})
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
	w = reqWithHeaders(t, h, "POST", "/public/v1/sales/"+id+"/lines", "a234567890123456", map[string]any{"line_id": "l1", "name": "Coffee", "quantity": "1.000", "unit_price": map[string]string{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"}, map[string]string{"If-Match": "1"})
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
