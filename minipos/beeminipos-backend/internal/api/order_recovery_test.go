package api

import (
	"encoding/json"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOrderRecoveryPermissionReadOnly(t *testing.T) {
	for _, state := range []string{"FISCAL_PENDING", "UNKNOWN", "COMPLETED", "REVERSED", "FAILED", "DRAFT", "OPEN"} {
		t.Run(state, func(t *testing.T) {
			h, token, calls := recoveryHandler(t, nil, nil, func(o *domain.Order) { o.FiscalRegisterID = "register"; o.Version = 7; o.State = state })
			r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+"/recovery", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			if r.Header.Get("X-App-Instance-Id") == "" {
				r.Header.Set("X-App-Instance-Id", "instance")
			}
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var raw map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &raw)
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if raw["order_id"] != recoveryOrderID || raw["shift_id"] != "shift" || raw["register_id"] != "register" || raw["operator_code"] != "operator" || raw["client_operation_id"] != recoveryOperationID || raw["receipt_session_id"] != recoverySessionID || raw["fiscal_sale_id"] != "sale" || raw["order_version"] != float64(7) {
				t.Fatalf("bad association %v", raw)
			}
			want := 1
			if state == "DRAFT" || state == "OPEN" {
				want = 0
			}
			actions := raw["allowed_actions"].([]any)
			if len(actions) != want || (want == 1 && actions[0] != "READ") || *calls != 0 {
				t.Fatalf("state%s actions%v upstream%d", state, actions, *calls)
			}
			q := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+"/recovery", nil)
			q.Header = r.Header.Clone()
			after := httptest.NewRecorder()
			h.ServeHTTP(after, q)
			var order map[string]any
			_ = json.Unmarshal(after.Body.Bytes(), &order)
			if order["state"] != state || order["order_version"] != float64(7) {
				t.Fatalf("permission mutated order %d %s", after.Code, after.Body.String())
			}
		})
	}
}
func TestOrderRecoveryPermissionDeniesMissingAssociation(t *testing.T) {
	for _, kind := range []string{"operation", "receipt", "conflict", "explicit-fiscal-register"} {
		t.Run(kind, func(t *testing.T) {
			h, token, calls := recoveryHandler(t, nil, nil, func(o *domain.Order) {
				o.FiscalRegisterID = "register"
				o.Version = 1
				if kind == "explicit-fiscal-register" {
					o.FiscalRegisterID = ""
				}
				if kind == "operation" {
					o.CheckoutOperationID = ""
				}
				if kind == "receipt" {
					o.ReceiptSessionID = ""
				}
				if kind == "conflict" {
					o.FiscalOperationID = "other"
				}
			})
			r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+"/recovery", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			if r.Header.Get("X-App-Instance-Id") == "" {
				r.Header.Set("X-App-Instance-Id", "instance")
			}
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var out struct {
				Allowed []string `json:"allowed_actions"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if w.Code != 200 || len(out.Allowed) != 0 || *calls != 0 {
				t.Fatalf("%d %s upstream%d", w.Code, w.Body.String(), *calls)
			}
		})
	}
}
func TestOrderRecoveryPermissionRequiresTenantActorSession(t *testing.T) {
	for _, kind := range []string{"tenant", "actor", "session", "instance", "anonymous"} {
		t.Run(kind, func(t *testing.T) {
			h, token, calls := recoveryHandler(t, nil, nil, func(o *domain.Order) {
				o.FiscalRegisterID = "register"
				o.Version = 1
				if kind == "explicit-fiscal-register" {
					o.FiscalRegisterID = ""
				}
				if kind == "tenant" {
					o.TenantID = "foreign"
				}
			})
			want := 401
			if kind == "tenant" {
				want = 404
			}
			if kind == "actor" || kind == "session" {
				subject := "operator"
				if kind == "actor" {
					subject = "foreign"
					want = 403
				}
				var err error
				token, err = auth.Sign(auth.Claims{Subject: subject, Issuer: "test", TenantID: "tenant-a", Roles: []string{"ADMIN"}, Scope: "fiscal.base", ExpiresAt: time.Now().Add(2 * time.Hour).Unix()}, []byte(testAuthHMACKey))
				if err != nil {
					t.Fatal(err)
				}
			}
			r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+"/recovery", nil)
			if kind != "anonymous" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			if kind == "instance" {
				r.Header.Set("X-App-Instance-Id", "foreign-instance")
			}
			if r.Header.Get("X-App-Instance-Id") == "" {
				r.Header.Set("X-App-Instance-Id", "instance")
			}
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != want || *calls != 0 {
				t.Fatalf("%s %d want%d %s upstream%d", kind, w.Code, want, w.Body.String(), *calls)
			}
		})
	}
}

func TestRecoveryPermissionUnlinkedSourceUsesAssociatedOperationLookupOnly(t *testing.T) {
	h, token, calls := recoveryHandler(t, nil, nil, func(o *domain.Order) { o.FiscalRegisterID = "register"; o.Version = 3; o.FiscalSaleID = "" })
	read := func(suffix string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+suffix, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-App-Instance-Id", "instance")
		r.Header.Set("X-Api-Version", "2026-08-07")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	before := read("/recovery")
	var grant map[string]any
	_ = json.Unmarshal(before.Body.Bytes(), &grant)
	if before.Code != 200 || grant["fiscal_sale_id"] != "" || len(grant["allowed_actions"].([]any)) != 1 || *calls != 0 {
		t.Fatalf("%d %s upstream%d", before.Code, before.Body.String(), *calls)
	}
	op := read("/operations/" + recoveryOperationID)
	if op.Code != 200 {
		t.Fatalf("associated source read failed%d %s", op.Code, op.Body.String())
	}
	after := read("/recovery")
	_ = json.Unmarshal(after.Body.Bytes(), &grant)
	if after.Code != 200 || grant["fiscal_sale_id"] != "sale" || grant["client_operation_id"] != recoveryOperationID || grant["receipt_session_id"] != recoverySessionID || *calls != 1 {
		t.Fatalf("wrong association%d %s upstream%d", after.Code, after.Body.String(), *calls)
	}
}
