package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type recoveryOutageStore struct {
	raw          []byte
	fail         bool
	saves, loads int
}

func (s *recoveryOutageStore) Load() ([]byte, error) {
	s.loads++
	if s.fail {
		return nil, errors.New("read unavailable")
	}
	return append([]byte(nil), s.raw...), nil
}
func (s *recoveryOutageStore) Save(b []byte) error {
	s.saves++
	if s.fail {
		return errors.New("commit outcome uncertain")
	}
	s.raw = append([]byte(nil), b...)
	return nil
}
func TestRecoveryPermissionPersistenceOutageFailsClosedAndCanRefreshReadOnly(t *testing.T) {
	order := domain.Order{ID: recoveryOrderID, TenantID: "tenant-a", ShiftID: "shift", RegisterID: "register", OperatorCode: "operator", ExternalID: "external", State: "UNKNOWN", FiscalRegisterID: "register", Version: 2, CheckoutOperationID: recoveryOperationID, ReceiptSessionID: recoverySessionID, FiscalSaleID: "sale"}
	raw, _ := json.Marshal(map[string]any{"products": map[string]any{}, "tax_groups": map[string]any{}, "checkouts": map[string]any{}, "configurations": map[string]any{}, "orders": map[string]domain.Order{order.ID: order}, "shifts": map[string]domain.Shift{"shift": {ID: "shift", TenantID: "tenant-a", EmployeeID: "employee"}}, "employees": map[string]domain.Employee{"employee": {ID: "employee", TenantID: "tenant-a", Active: true, Status: "ACTIVE"}}})
	store := &recoveryOutageStore{raw: raw}
	s, err := domain.NewPersistentService("https://unused", "2026-08-07", store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindEmployeeIdentity("tenant-a", "employee", "operator", "test"); err != nil {
		t.Fatal(err)
	}
	token, err := auth.Sign(auth.Claims{Subject: "operator", Issuer: "test", TenantID: "tenant-a", Roles: []string{"CASHIER"}, Scope: "fiscal.base", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte(testAuthHMACKey))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(token))
	if _, err = s.RegisterOperatorSession("tenant-a", "employee", "instance", base64.RawURLEncoding.EncodeToString(digest[:]), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	store.fail = true
	if _, err = s.RegisterOperatorSession("tenant-a", "employee", "other", "other-token", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected uncertain save")
	}
	saves := store.saves
	h := auth.Middleware(testAuthHMACKey, enforceSuccessResponses(http.HandlerFunc((&handler{s: s}).order)))
	read := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+order.ID+"/recovery", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-App-Instance-Id", "instance")
		r.Header.Set("X-Api-Version", "2026-08-07")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	denied := read()
	if denied.Code != 503 {
		t.Fatalf("stale acknowledged state issued capability: %d %s", denied.Code, denied.Body.String())
	}
	if store.saves != saves {
		t.Fatal("permission refresh wrote during outage")
	}
	store.fail = false
	allowed := read()
	var body struct {
		Actions []string `json:"allowed_actions"`
	}
	_ = json.Unmarshal(allowed.Body.Bytes(), &body)
	if allowed.Code != 200 || len(body.Actions) != 1 || body.Actions[0] != "READ" {
		t.Fatalf("authoritative read did not recover capability: %d %s", allowed.Code, allowed.Body.String())
	}
	if store.saves != saves {
		t.Fatal("readiness refresh persisted or retried mutation")
	}
}

type recoveryHookStore struct {
	recoveryOutageStore
	onShiftRead func()
}

func (s *recoveryHookStore) LoadTenantEntity(collection, tenant, id string) ([]byte, error) {
	if collection == "shifts" && s.onShiftRead != nil {
		hook := s.onShiftRead
		s.onShiftRead = nil
		hook()
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(s.raw, &snapshot); err != nil {
		return nil, err
	}
	var rows map[string]json.RawMessage
	if err := json.Unmarshal(snapshot[collection], &rows); err != nil {
		return nil, err
	}
	raw := rows[id]
	if raw == nil {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), raw...), nil
}
func (s *recoveryHookStore) LoadTenantEntities(collection, tenant string) ([][]byte, error) {
	return nil, nil
}
func TestRecoveryPermissionMutationDuringActorAuthorizationInvalidatesSnapshot(t *testing.T) {
	for _, outage := range []bool{false, true} {
		t.Run(map[bool]string{false: "successful unrelated mutation", true: "uncertain save and reload outage"}[outage], func(t *testing.T) {
			order := domain.Order{ID: recoveryOrderID, TenantID: "tenant-a", ShiftID: "shift", RegisterID: "register", OperatorCode: "operator", ExternalID: "external", State: "UNKNOWN", FiscalRegisterID: "register", Version: 2, CheckoutOperationID: recoveryOperationID, ReceiptSessionID: recoverySessionID, FiscalSaleID: "sale"}
			raw, _ := json.Marshal(map[string]any{"products": map[string]any{}, "tax_groups": map[string]any{}, "checkouts": map[string]any{}, "configurations": map[string]any{}, "orders": map[string]domain.Order{order.ID: order}, "shifts": map[string]domain.Shift{"shift": {ID: "shift", TenantID: "tenant-a", EmployeeID: "employee", State: "CLOSED"}}, "employees": map[string]domain.Employee{"employee": {ID: "employee", TenantID: "tenant-a", Active: true, Status: "ACTIVE"}}})
			store := &recoveryHookStore{recoveryOutageStore: recoveryOutageStore{raw: raw}}
			service, err := domain.NewPersistentService("https://unused", "2026-08-07", store)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = service.BindEmployeeIdentity("tenant-a", "employee", "operator", "test"); err != nil {
				t.Fatal(err)
			}
			token, err := auth.Sign(auth.Claims{Subject: "operator", Issuer: "test", TenantID: "tenant-a", Roles: []string{"CASHIER"}, Scope: "fiscal.base", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte(testAuthHMACKey))
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(token))
			if _, err = service.RegisterOperatorSession("tenant-a", "employee", "instance", base64.RawURLEncoding.EncodeToString(digest[:]), time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			before := store.saves
			fired := false
			store.onShiftRead = func() {
				fired = true
				store.fail = outage
				_, e := service.RegisterOperatorSession("tenant-a", "employee", "unrelated-instance", "unrelated-token", time.Now().Add(time.Hour))
				if (e != nil) != outage {
					t.Errorf("mutation error=%v outage=%v", e, outage)
				}
			}
			request := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+order.ID+"/recovery", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("X-App-Instance-Id", "instance")
			request.Header.Set("X-Api-Version", "2026-08-07")
			recorder := httptest.NewRecorder()
			auth.Middleware(testAuthHMACKey, enforceSuccessResponses(http.HandlerFunc((&handler{s: service}).order))).ServeHTTP(recorder, request)
			if !fired {
				t.Fatalf("authorization hook was not exercised: %d %s", recorder.Code, recorder.Body.String())
			}
			if recorder.Code != 503 {
				t.Fatalf("stale recovery snapshot granted after intervening mutation: %d %s", recorder.Code, recorder.Body.String())
			}
			if store.saves != before+1 {
				t.Fatalf("recovery endpoint introduced extra persistence: before=%d after=%d", before, store.saves)
			}
		})
	}
}
