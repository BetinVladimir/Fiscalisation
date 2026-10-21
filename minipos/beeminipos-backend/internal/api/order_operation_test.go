package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type operationReadFunc func(context.Context, string, string) (fiscalintegration.FiscalOperation, error)

func (f operationReadFunc) GetFiscalOperation(c context.Context, t, id string) (fiscalintegration.FiscalOperation, error) {
	return f(c, t, id)
}

func (f operationReadFunc) GetFiscalSale(ctx context.Context, company, id string) (fiscalintegration.FiscalSale, error) {
	sale := fiscalintegration.FiscalSale{SaleID: id, TenantID: "fiscal-tenant-a", ExternalID: "external-order", RegisterID: "register", OperatorID: "operator"}
	switch id {
	case "foreign-sale":
		sale.ExternalID = "foreign-order"
	case "foreign-register":
		sale.RegisterID = "foreign"
	case "foreign-operator":
		sale.OperatorID = "foreign"
	case "foreign-tenant":
		sale.TenantID = "foreign"
	}
	return sale, nil
}

const recoveryOrderID = "00000000-0000-4000-8000-000000000021"
const recoveryOperationID = "00000000-0000-4000-8000-000000000022"
const recoverySessionID = "00000000-0000-4000-8000-000000000023"

func recoveryHandler(t *testing.T, mutate func(*fiscalintegration.FiscalOperation), upstreamErr error, edit ...func(*domain.Order)) (http.Handler, string, *int) {
	t.Helper()
	order := domain.Order{ID: recoveryOrderID, TenantID: "tenant-a", ShiftID: "shift", RegisterID: "register", CheckoutOperationID: recoveryOperationID, ReceiptSessionID: recoverySessionID, FiscalSaleID: "sale", State: "UNKNOWN", ExternalID: "external-order", OperatorCode: "operator"}
	if len(edit) > 0 {
		edit[0](&order)
	}
	data, _ := json.Marshal(map[string]any{"orders": map[string]domain.Order{recoveryOrderID: order}, "shifts": map[string]domain.Shift{order.ShiftID: {ID: order.ShiftID, TenantID: "tenant-a", EmployeeID: "employee"}}, "employees": map[string]domain.Employee{"employee": {ID: "employee", TenantID: "tenant-a", Active: true, Status: "ACTIVE"}}})
	svc, err := domain.NewPersistentService("https://unused", "2026-08-07", &apiStore{b: data})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.BindEmployeeIdentity("tenant-a", "employee", "operator", "test"); err != nil {
		t.Fatal(err)
	}
	token, err := auth.Sign(auth.Claims{Subject: "operator", Issuer: "test", TenantID: "tenant-a", Roles: []string{"CASHIER"}, Scope: "fiscal.base", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte(testAuthHMACKey))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(token))
	if _, err = svc.RegisterOperatorSession("tenant-a", "employee", "instance", base64.RawURLEncoding.EncodeToString(digest[:]), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	calls := new(int)
	h := &handler{s: svc, operations: operationReadFunc(func(ctx context.Context, tenant, id string) (fiscalintegration.FiscalOperation, error) {
		*calls++
		if tenant != "tenant-a" || id != recoveryOperationID {
			t.Error("incorrect lookup scope")
		}
		op := fiscalintegration.FiscalOperation{OperationID: id, ClientOperationID: id, ReceiptSessionID: recoverySessionID, TenantID: "fiscal-tenant-a", SaleID: "sale", RegisterID: "register", Type: "SALE_FINALIZE", State: "FISCALIZED", Version: 1, FiscalReference: "receipt-17", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if mutate != nil {
			mutate(&op)
		}
		return op, upstreamErr
	})}
	return auth.Middleware(testAuthHMACKey, enforceSuccessResponses(http.HandlerFunc(h.order))), token, calls
}
func TestCloudOperationRecoveryReadOnlyAssociation(t *testing.T) {
	for _, field := range []string{"success", "uuid", "client", "session", "sale", "register", "type", "version", "reference", "upstream"} {
		t.Run(field, func(t *testing.T) {
			var upstreamErr error
			if field == "upstream" {
				upstreamErr = errors.New("timeout")
			}
			h, token, calls := recoveryHandler(t, func(op *fiscalintegration.FiscalOperation) {
				switch field {
				case "uuid":
					op.OperationID = "other"
				case "client":
					op.ClientOperationID = "other"
				case "session":
					op.ReceiptSessionID = "other"
				case "sale":
					op.SaleID = "other"
				case "register":
					op.RegisterID = "other"
				case "type":
					op.Type = "REVERSAL"
				case "version":
					op.Version = 0
				case "reference":
					op.FiscalReference = ""
				}
			}, upstreamErr)
			r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+"/operations/"+recoveryOperationID, nil)
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-App-Instance-Id", "instance")
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := 502
			if field == "success" {
				want = 200
			}
			if w.Code != want || *calls != 1 {
				t.Fatalf("got %d calls%d body%s", w.Code, *calls, w.Body.String())
			}
		})
	}
}
func TestCloudOperationRecoveryRejectsWrongActorOrAssociationBeforeLookup(t *testing.T) {
	for _, scenario := range []string{"wrong uuid", "missing order", "missing session", "wrong actor", "foreign tenant"} {
		t.Run(scenario, func(t *testing.T) {
			h, token, calls := recoveryHandler(t, nil, nil)
			orderID, opID := recoveryOrderID, recoveryOperationID
			if scenario == "wrong uuid" {
				opID = "00000000-0000-4000-8000-000000000099"
			}
			if scenario == "missing order" {
				orderID = "00000000-0000-4000-8000-000000000099"
			}
			if scenario == "wrong actor" || scenario == "foreign tenant" {
				tenant, sub := "tenant-a", "different"
				if scenario == "foreign tenant" {
					tenant = "tenant-b"
				}
				token, _ = auth.Sign(auth.Claims{Subject: sub, Issuer: "test", TenantID: tenant, Roles: []string{"CASHIER"}, Scope: "fiscal.base", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte(testAuthHMACKey))
			}
			r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+orderID+"/operations/"+opID, nil)
			r.Header.Set("Authorization", "Bearer "+token)
			if scenario != "missing session" {
				r.Header.Set("X-App-Instance-Id", "instance")
			}
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code < 400 || *calls != 0 {
				t.Fatalf("got %d calls%d body%s", w.Code, *calls, w.Body.String())
			}
		})
	}
}

func TestCloudOperationRecoveryBindsMissingSaleByReadOnlyProof(t *testing.T) {
	for _, scenario := range []string{"success", "foreign-sale", "foreign-register", "foreign-operator", "foreign-tenant", "absent operation"} {
		t.Run(scenario, func(t *testing.T) {
			var upstream error
			if scenario == "absent operation" {
				upstream = errors.New("404")
			}
			h, token, calls := recoveryHandler(t, func(op *fiscalintegration.FiscalOperation) {
				if strings.HasPrefix(scenario, "foreign-") {
					op.SaleID = scenario
				}
			}, upstream, func(order *domain.Order) { order.FiscalSaleID = "" })
			r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+"/operations/"+recoveryOperationID, nil)
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-App-Instance-Id", "instance")
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := 502
			if scenario == "success" {
				want = 200
			}
			if w.Code != want || *calls != 1 {
				t.Fatalf("code%d body%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCloudLookupReturnsEffectivePriorProofAfterUnknownRead(t *testing.T) {
	h, token, _ := recoveryHandler(t, func(op *fiscalintegration.FiscalOperation) { op.State = "UNKNOWN"; op.FiscalReference = "" }, nil, func(order *domain.Order) {
		order.State = "COMPLETED"
		order.ReceiptReference = "prior-proven-receipt"
		order.FiscalVersion = 2
	})
	r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+"/operations/"+recoveryOperationID, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-App-Instance-Id", "instance")
	r.Header.Set("X-Api-Version", "2026-08-07")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result fiscalintegration.FiscalOperation
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.State != "FISCALIZED" || result.FiscalReference != "prior-proven-receipt" || result.Version != 2 {
		t.Fatal(result, err)
	}
}

func TestCloudOperationLookupMakesFinalFailureAuthoritativeForSourceReload(t *testing.T) {
	for _, state := range []string{"FAILED", "CANCELLED"} {
		t.Run(state, func(t *testing.T) {
			h, token, calls := recoveryHandler(t, func(op *fiscalintegration.FiscalOperation) { op.State = state; op.FiscalReference = "" }, nil, func(order *domain.Order) {
				order.Version = 4
				order.ShiftID = "00000000-0000-4000-8000-000000000024"
				order.Lines = []domain.Line{}
				order.Total = domain.Money{Amount: "2.50", Currency: "EUR"}
				order.CreatedAt = time.Now().UTC()
				order.UpdatedAt = order.CreatedAt
			})
			read := func(path string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("GET", "/public/v1/minipos/orders/"+recoveryOrderID+path, nil)
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("X-App-Instance-Id", "instance")
				r.Header.Set("X-Api-Version", "2026-08-07")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w
			}
			for attempt := 0; attempt < 2; attempt++ {
				operation := read("/operations/" + recoveryOperationID)
				if operation.Code != 200 {
					t.Fatal("operation lookup failed", operation.Code, operation.Body.String())
				}
				response := read("")
				if response.Code != 200 {
					t.Fatal("source reload failed", response.Code, response.Body.String())
				}
				var order domain.Order
				if err := json.Unmarshal(response.Body.Bytes(), &order); err != nil {
					t.Fatal(err)
				}
				if order.State != "FAILED" || order.FiscalOperationID != recoveryOperationID || order.CheckoutOperationID != recoveryOperationID || order.ReceiptSessionID != recoverySessionID || order.FiscalSaleID != "sale" || order.ReceiptReference != "" || order.FiscalVersion != 1 || order.Version != 5 {
					t.Fatal("source failure was not durable/idempotent", order)
				}
				var payload struct {
					AllowedActions []string `json:"allowed_actions"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || len(payload.AllowedActions) != 0 {
					t.Fatal("failed source order remains payable", response.Body.String(), err)
				}
			}
			if *calls != 2 {
				t.Fatal("lookup dispatched unexpected upstream commands", *calls)
			}
		})
	}
}
