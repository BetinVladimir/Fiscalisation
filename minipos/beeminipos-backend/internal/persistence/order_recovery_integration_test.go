package persistence_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fiscalisation/beeminipos-backend/internal/api"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/persistence"
)

func typedRecoveryUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		t.Fatal(e)
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	v := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", v[:8], v[8:12], v[12:16], v[16:20], v[20:])
}
func TestTypedOrderRecoveryHTTPAndPrivateProjection(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set; requires disposable fixture")
	}
	db, e := sql.Open("pgx", url)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	store, e := persistence.OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	var rootCalls atomic.Int32
	root := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rootCalls.Add(1); w.WriteHeader(500) }))
	defer root.Close()
	s, e := domain.NewPersistentService(root.URL+"/public/v1", "2026-08-07", store)
	if e != nil {
		t.Fatal(e)
	}
	tenant := "typed-recovery-" + typedRecoveryUUID(t)
	sourceRegister, rootRegister := typedRecoveryUUID(t), typedRecoveryUUID(t)
	employee, e := s.CreateEmployee(domain.Employee{TenantID: tenant, FirstName: "Synthetic", LastName: "Recovery", OperatorCode: "A001", Roles: []string{"CASHIER"}, Status: "ACTIVE"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.BindEmployeeIdentity(tenant, employee.ID, "synthetic-operator", "synthetic-issuer"); e != nil {
		t.Fatal(e)
	}
	const key = "synthetic-recovery-test-only-key-32bytes"
	token, e := auth.Sign(auth.Claims{Subject: "synthetic-operator", Issuer: "synthetic-issuer", TenantID: tenant, Roles: []string{"CASHIER"}, Scope: "fiscal.base", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte(key))
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256([]byte(token))
	if _, e = s.RegisterOperatorSession(tenant, employee.ID, "synthetic-instance", base64.RawURLEncoding.EncodeToString(digest[:]), time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	shift, e := s.OpenShiftForTenant(sourceRegister, employee.ID, tenant)
	if e != nil {
		t.Fatal(e)
	}
	order, e := s.CreateOrder(domain.Order{TenantID: tenant, ShiftID: shift.ID})
	if e != nil {
		t.Fatal(e)
	}
	order, e = s.AddLineExpectedForTenant(order.ID, order.Version, domain.Line{LineID: typedRecoveryUUID(t), Name: "Synthetic item", Quantity: "1.000", UnitPrice: domain.Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}, tenant)
	if e != nil {
		t.Fatal(e)
	}
	paymentID := typedRecoveryUUID(t)
	payments := []map[string]any{{"payment_id": paymentID, "type": "CASH", "amount": map[string]any{"amount": "2.50", "currency": "EUR"}}}
	order, e = s.FreezeExecutionOrder(order.ID, shift.ID, employee.ID, tenant, payments)
	if e != nil {
		t.Fatal(e)
	}
	operation, receipt := typedRecoveryUUID(t), typedRecoveryUUID(t)
	const privateAuthorization = "synthetic-private-preparation-id-do-not-project"
	order, e = s.AttachPreparedFiscalExecution(order.ID, tenant, "synthetic-root-sale", operation, receipt, rootRegister, privateAuthorization)
	if e != nil {
		t.Fatal(e)
	}
	var original []byte
	if e = db.QueryRow(`select payload from minipos.minipos_runtime_orders where id=$1`, order.ID).Scan(&original); e != nil {
		t.Fatal(e)
	}
	var base map[string]any
	if e = json.Unmarshal(original, &base); e != nil {
		t.Fatal(e)
	}
	restore := func(payload map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(payload)
		money, _ := json.Marshal(base["payments"])
		lines, _ := json.Marshal(base["lines"])
		_, err := db.Exec(`update minipos.minipos_runtime_orders set payload=$2::jsonb,total=2.50,payments=$3::jsonb,lines=$4::jsonb,version=$5,state=$6 where id=$1`, order.ID, string(raw), string(money), string(lines), order.Version, order.State)
		if err != nil {
			t.Fatal(err)
		}
	}
	clone := func() map[string]any { var x map[string]any; _ = json.Unmarshal(original, &x); return x }
	h := api.New(s, config.Config{APIVersion: "2026-08-07", AuthHMACKey: key})
	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		var before, after int64
		if e = db.QueryRow(`select generation from minipos.minipos_state_meta where singleton=true`).Scan(&before); e != nil {
			t.Fatal(e)
		}
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-App-Instance-Id", "synthetic-instance")
		r.Header.Set("X-Api-Version", "2026-08-07")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if e = db.QueryRow(`select generation from minipos.minipos_state_meta where singleton=true`).Scan(&after); e != nil {
			t.Fatal(e)
		}
		if before != after || rootCalls.Load() != 0 {
			t.Fatal("recovery GET wrote persistence or called fiscal root")
		}
		return w
	}
	recoveryPath := "/public/v1/minipos/orders/" + order.ID + "/recovery"
	t.Run("actual-GET-prepared-grant", func(t *testing.T) {
		restore(clone())
		w := get(recoveryPath)
		var grant struct {
			Actions   []string `json:"allowed_actions"`
			Register  string   `json:"fiscal_register_id"`
			Operation string   `json:"client_operation_id"`
			Receipt   string   `json:"receipt_session_id"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &grant)
		if w.Code != 200 || len(grant.Actions) != 1 || grant.Actions[0] != "READ" || grant.Register != rootRegister || grant.Operation != operation || grant.Receipt != receipt {
			t.Fatalf("actual GET /orders/id/recovery missing typed prepared grant: %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("prepared-public-positive", func(t *testing.T) {
		restore(clone())
		for _, read := range []func() ([]byte, error){func() ([]byte, error) { return store.LoadTenantEntity("orders", tenant, order.ID) }, func() ([]byte, error) {
			rows, err := store.LoadTenantEntities("orders", tenant)
			if err != nil {
				return nil, err
			}
			if len(rows) != 1 {
				return nil, fmt.Errorf("expected one order, got%d", len(rows))
			}
			return rows[0], nil
		}} {
			raw, err := read()
			if err != nil {
				t.Fatal(err)
			}
			var projected domain.Order
			if err = json.Unmarshal(raw, &projected); err != nil {
				t.Fatal(err)
			}
			if projected.CheckoutOperationID != operation || projected.ReceiptSessionID != receipt || projected.FiscalRegisterID != rootRegister || projected.Total != order.Total || len(projected.Payments) != 1 || projected.Payments[0].PaymentID != paymentID {
				t.Fatalf("typed prepared association/tender lost: %s", raw)
			}
			if bytes.Contains(raw, []byte(privateAuthorization)) || bytes.Contains(raw, []byte("execution_authorization_id")) {
				t.Fatal("private authorization projected")
			}
		}
		w := get(recoveryPath)
		var grant struct {
			Actions   []string `json:"allowed_actions"`
			Register  string   `json:"fiscal_register_id"`
			Operation string   `json:"client_operation_id"`
			Receipt   string   `json:"receipt_session_id"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &grant)
		if w.Code != 200 || len(grant.Actions) != 1 || grant.Actions[0] != "READ" || grant.Register != rootRegister || grant.Operation != operation || grant.Receipt != receipt {
			t.Fatalf("actual typed PG recovery grant failed: %d %s", w.Code, w.Body.String())
		}
		w = get("/public/v1/minipos/orders/" + order.ID)
		var public map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &public)
		confirmation, ok := public["checkout_confirmation"].(map[string]any)
		encoded, _ := json.Marshal(payments)
		expected := sha256.Sum256(encoded)
		if w.Code != 200 || !ok || confirmation["root_register_id"] != rootRegister || confirmation["payments_sha256"] != hex.EncodeToString(expected[:]) {
			t.Fatalf("public prepared tender commitment missing: %d %s", w.Code, w.Body.String())
		}
		for _, private := range []string{privateAuthorization, "execution_authorization_id", "\"payments\"", "card_number", "customer"} {
			if strings.Contains(w.Body.String(), private) {
				t.Fatalf("private public-order value leaked: %s", private)
			}
		}
	})
	for _, missing := range []string{"checkout_operation_id", "receipt_session_id", "fiscal_register_id"} {
		t.Run("missing-legacy-"+missing, func(t *testing.T) {
			x := clone()
			delete(x, missing)
			restore(x)
			w := get(recoveryPath)
			var grant struct {
				Actions []string `json:"allowed_actions"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &grant)
			if w.Code != 200 || len(grant.Actions) != 0 {
				t.Fatalf("missing metadata granted READ: %d %s", w.Code, w.Body.String())
			}
		})
	}
	tests := map[string]func(map[string]any){"number-receipt": func(x map[string]any) { x["receipt_session_id"] = 123 }, "null-register": func(x map[string]any) { x["fiscal_register_id"] = nil }, "object-checkout": func(x map[string]any) { x["checkout_operation_id"] = map[string]any{"id": operation} }, "wrong-operation": func(x map[string]any) { x["checkout_operation_id"] = typedRecoveryUUID(t) }, "wrong-source-id": func(x map[string]any) { x["id"] = "another-source" }, "wrong-tenant": func(x map[string]any) { x["tenant_id"] = "another-tenant" }, "wrong-source-register": func(x map[string]any) { x["register_id"] = rootRegister }, "wrong-state": func(x map[string]any) { x["state"] = "COMPLETED" }, "wrong-version": func(x map[string]any) { x["version"] = 999 }, "wrong-sale": func(x map[string]any) { x["fiscal_sale_id"] = "another-sale" }, "wrong-total": func(x map[string]any) { x["total"].(map[string]any)["amount"] = "9.99" }, "wrong-tender": func(x map[string]any) { x["payments"].([]any)[0].(map[string]any)["type"] = "CARD" }}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			x := clone()
			change(x)
			restore(x)
			if _, err := store.LoadTenantEntity("orders", tenant, order.ID); err == nil {
				t.Fatal("corrupt typed projection read succeeded")
			}
			if _, err := store.LoadTenantEntities("orders", tenant); err == nil {
				t.Fatal("corrupt typed list succeeded")
			}
			w := get(recoveryPath)
			if w.Code != 503 || strings.Contains(w.Body.String(), `"READ"`) {
				t.Fatalf("corrupt/mismatched metadata issued capability: %d %s", w.Code, w.Body.String())
			}
		})
	}
	t.Run("no-private-payload-merge", func(t *testing.T) {
		x := clone()
		x["client_proof"] = "PRIVATE_PROOF_SENTINEL"
		x["customer"] = "PRIVATE_CUSTOMER_SENTINEL"
		x["signed_intent"] = "PRIVATE_SIGNATURE_SENTINEL"
		restore(x)
		raw, err := store.LoadTenantEntity("orders", tenant, order.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"PRIVATE_", privateAuthorization, "execution_authorization_id"} {
			if bytes.Contains(raw, []byte(secret)) {
				t.Fatal("payload merged private fields")
			}
		}
		w := get(recoveryPath)
		if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE_") {
			t.Fatalf("private leak or missing grant: %d %s", w.Code, w.Body.String())
		}
	})
	for _, invalid := range []string{"missing-payments", "empty-payments", "bad-payment-type", "bad-payment-id", "bad-payment-sum", "bad-payment-currency"} {
		t.Run("prepared-plan-"+invalid, func(t *testing.T) {
			x := clone()
			switch invalid {
			case "missing-payments":
				delete(x, "payments")
			case "empty-payments":
				x["payments"] = []any{}
			default:
				p := x["payments"].([]any)[0].(map[string]any)
				switch invalid {
				case "bad-payment-type":
					p["type"] = "UNKNOWN"
				case "bad-payment-id":
					p["payment_id"] = "not-a-uuid"
				case "bad-payment-sum":
					p["amount"].(map[string]any)["amount"] = "2.49"
				case "bad-payment-currency":
					p["amount"].(map[string]any)["currency"] = "USD"
				}
			}
			restore(x)
			values, ok := x["payments"]
			if !ok {
				values = []any{}
			}
			raw, _ := json.Marshal(values)
			if _, err := db.Exec(`update minipos.minipos_runtime_orders set payments=$2::jsonb where id=$1`, order.ID, string(raw)); err != nil {
				t.Fatal(err)
			}
			w := get(recoveryPath)
			if w.Code != 503 || strings.Contains(w.Body.String(), `"READ"`) {
				t.Fatalf("invalid fully prepared plan granted capability: %d %s", w.Code, w.Body.String())
			}
		})
	}
	t.Run("nested-private-metadata-whitelist", func(t *testing.T) {
		x := clone()
		payment := x["payments"].([]any)[0].(map[string]any)
		payment["card_number"] = "PRIVATE_CARD_SENTINEL"
		payment["amount"].(map[string]any)["customer"] = "PRIVATE_CUSTOMER_SENTINEL"
		line := x["lines"].([]any)[0].(map[string]any)
		line["customer"] = "PRIVATE_LINE_CUSTOMER_SENTINEL"
		restore(x)
		rawPayments, _ := json.Marshal(x["payments"])
		rawLines, _ := json.Marshal(x["lines"])
		if _, err := db.Exec(`update minipos.minipos_runtime_orders set payments=$2::jsonb,lines=$3::jsonb where id=$1`, order.ID, string(rawPayments), string(rawLines)); err != nil {
			t.Fatal(err)
		}
		raw, err := store.LoadTenantEntity("orders", tenant, order.ID)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("PRIVATE_")) {
			t.Fatal("nested private metadata escaped whitelist")
		}
		w := get(recoveryPath)
		if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE_") {
			t.Fatalf("private nested recovery projection: %d %s", w.Code, w.Body.String())
		}
	})
	restore(clone())
	if _, err := store.LoadTenantEntity("orders", "another-tenant", order.ID); err == nil {
		t.Fatal("typed association bypassed tenant RLS")
	}
}
