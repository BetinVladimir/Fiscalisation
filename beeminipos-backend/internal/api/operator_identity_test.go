package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func operatorToken(t *testing.T, secret, subject, issuer, tenant, role string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload, err := json.Marshal(map[string]any{"sub": subject, "iss": issuer, "tenant_id": tenant, "roles": []string{role}, "scope": "fiscal.base", "exp": time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func operatorRequest(h http.Handler, method, path, token, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Api-Version", "2026-08-07")
	r.Header.Set("X-App-Instance-Id", "00000000-0000-4000-8000-000000000001")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestOperatorIdentityBindingAndShiftFailClosed(t *testing.T) {
	const secret, issuer, tenant = "operator-session-secret", "https://identity.example.test", "org-identity"
	h := New(domain.NewService("http://invalid", "2026-08-07"), config.Config{AppEnv: "dev", APIVersion: "2026-08-07", AuthHMACKey: secret})
	admin := operatorToken(t, secret, "admin-1", issuer, tenant, "ADMIN")
	cashier := operatorToken(t, secret, "cashier-1", issuer, tenant, "CASHIER")
	other := operatorToken(t, secret, "cashier-2", issuer, tenant, "CASHIER")

	created := operatorRequest(h, http.MethodPost, "/public/v1/minipos/employees", admin, "employee-create-01", `{"first_name":"Ada","last_name":"Lovelace","operator_code":"A001"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("employee: %d %s", created.Code, created.Body.String())
	}
	var employee domain.Employee
	if err := json.Unmarshal(created.Body.Bytes(), &employee); err != nil {
		t.Fatal(err)
	}

	deniedBinding := operatorRequest(h, http.MethodPost, "/public/v1/minipos/employees/"+employee.ID+"/identity-binding", cashier, "binding-denied-01", `{"subject":"cashier-1","issuer":"https://identity.example.test"}`)
	if deniedBinding.Code != http.StatusForbidden {
		t.Fatalf("cashier created identity binding: %d %s", deniedBinding.Code, deniedBinding.Body.String())
	}
	bound := operatorRequest(h, http.MethodPost, "/public/v1/minipos/employees/"+employee.ID+"/identity-binding", admin, "binding-create-01", `{"subject":"cashier-1","issuer":"https://identity.example.test"}`)
	if bound.Code != http.StatusCreated || bytes.Contains(bound.Body.Bytes(), []byte("cashier-1")) || bytes.Contains(bound.Body.Bytes(), []byte("subject_hash")) {
		t.Fatalf("binding response leaked subject or failed: %d %s", bound.Code, bound.Body.String())
	}

	session := operatorRequest(h, http.MethodGet, "/public/v1/minipos/operator-session", cashier, "", "")
	if session.Code != http.StatusOK || !bytes.Contains(session.Body.Bytes(), []byte(employee.ID)) {
		t.Fatalf("bound session: %d %s", session.Code, session.Body.String())
	}
	wrongInstance := httptest.NewRequest(http.MethodPost, "/public/v1/minipos/shifts", bytes.NewBufferString(`{"register_id":"00000000-0000-4000-8000-000000000001","employee_id":"`+employee.ID+`"}`))
	wrongInstance.Header.Set("Content-Type", "application/json")
	wrongInstance.Header.Set("Authorization", "Bearer "+cashier)
	wrongInstance.Header.Set("X-Api-Version", "2026-08-07")
	wrongInstance.Header.Set("X-App-Instance-Id", "00000000-0000-4000-8000-000000000099")
	wrongInstance.Header.Set("Idempotency-Key", "shift-wrong-app-01")
	wrongInstanceResponse := httptest.NewRecorder()
	h.ServeHTTP(wrongInstanceResponse, wrongInstance)
	if wrongInstanceResponse.Code != http.StatusUnauthorized {
		t.Fatalf("cross-app-instance shift accepted: %d %s", wrongInstanceResponse.Code, wrongInstanceResponse.Body.String())
	}
	unbound := operatorRequest(h, http.MethodGet, "/public/v1/minipos/operator-session", other, "", "")
	if unbound.Code != http.StatusForbidden {
		t.Fatalf("unbound identity session accepted: %d %s", unbound.Code, unbound.Body.String())
	}

	wrongShift := operatorRequest(h, http.MethodPost, "/public/v1/minipos/shifts", other, "shift-wrong-user-01", `{"register_id":"00000000-0000-4000-8000-000000000001","employee_id":"`+employee.ID+`"}`)
	if wrongShift.Code != http.StatusForbidden {
		t.Fatalf("cross-employee shift accepted: %d %s", wrongShift.Code, wrongShift.Body.String())
	}
	ownShift := operatorRequest(h, http.MethodPost, "/public/v1/minipos/shifts", cashier, "shift-own-user-0001", `{"register_id":"00000000-0000-4000-8000-000000000001","employee_id":"`+employee.ID+`"}`)
	if ownShift.Code != http.StatusCreated {
		t.Fatalf("bound employee shift rejected: %d %s", ownShift.Code, ownShift.Body.String())
	}
	var openedShift domain.Shift
	if err := json.Unmarshal(ownShift.Body.Bytes(), &openedShift); err != nil {
		t.Fatal(err)
	}
	createdOrder := operatorRequest(h, http.MethodPost, "/public/v1/minipos/orders", cashier, "discount-order-001", `{"shift_id":"`+openedShift.ID+`"}`)
	if createdOrder.Code != http.StatusCreated {
		t.Fatalf("discount test order rejected: %d %s", createdOrder.Code, createdOrder.Body.String())
	}
	var order domain.Order
	if err := json.Unmarshal(createdOrder.Body.Bytes(), &order); err != nil {
		t.Fatal(err)
	}
	discountRequest := httptest.NewRequest(http.MethodPost, "/public/v1/minipos/orders/"+order.ID+"/lines", bytes.NewBufferString(`{"line_id":"00000000-0000-4000-8000-000000000099","name":"Coffee","quantity":"1.000","unit_price":{"amount":"2.50","currency":"EUR"},"discount":{"amount":"0.20","currency":"EUR"},"tax_group":"B"}`))
	discountRequest.Header.Set("Content-Type", "application/json")
	discountRequest.Header.Set("Authorization", "Bearer "+cashier)
	discountRequest.Header.Set("X-Api-Version", "2026-08-07")
	discountRequest.Header.Set("X-App-Instance-Id", "00000000-0000-4000-8000-000000000001")
	discountRequest.Header.Set("Idempotency-Key", "discount-line-001")
	discountRequest.Header.Set("If-Match", "1")
	deniedDiscount := httptest.NewRecorder()
	h.ServeHTTP(deniedDiscount, discountRequest)
	if deniedDiscount.Code != http.StatusForbidden {
		t.Fatalf("cashier discount accepted: %d %s", deniedDiscount.Code, deniedDiscount.Body.String())
	}
	ownRecovery := operatorRequest(h, http.MethodGet, "/public/v1/minipos/shifts?employee_id="+employee.ID+"&register_id=00000000-0000-4000-8000-000000000001&state=OPEN", cashier, "", "")
	if ownRecovery.Code != http.StatusOK || !bytes.Contains(ownRecovery.Body.Bytes(), []byte(employee.ID)) {
		t.Fatalf("operator could not recover own shift: %d %s", ownRecovery.Code, ownRecovery.Body.String())
	}
	crossRecovery := operatorRequest(h, http.MethodGet, "/public/v1/minipos/shifts?employee_id="+employee.ID+"&register_id=00000000-0000-4000-8000-000000000001&state=OPEN", other, "", "")
	if crossRecovery.Code != http.StatusForbidden {
		t.Fatalf("foreign operator recovered shift: %d %s", crossRecovery.Code, crossRecovery.Body.String())
	}
	logout := operatorRequest(h, http.MethodPost, "/public/v1/minipos/operator-session", cashier, "operator-logout-01", `{}`)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout was not durably accepted: %d %s", logout.Code, logout.Body.String())
	}
	afterLogout := operatorRequest(h, http.MethodGet, "/public/v1/minipos/operator-session", cashier, "", "")
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token remained usable: %d %s", afterLogout.Code, afterLogout.Body.String())
	}
}

func TestProductionActorAuthorizationNeverFallsBackToAnonymous(t *testing.T) {
	svc := domain.NewService("http://invalid", "2026-08-07")
	employee, err := svc.CreateEmployee(domain.Employee{TenantID: "", FirstName: "A", LastName: "B", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	h := &handler{s: svc, c: config.Config{AppEnv: "prod"}}
	r := httptest.NewRequest(http.MethodPost, "/public/v1/minipos/shifts", nil)
	w := httptest.NewRecorder()
	if h.authorizeEmployeeActor(w, r, employee.ID) || w.Code != http.StatusUnauthorized {
		t.Fatalf("PROD anonymous actor accepted: %d", w.Code)
	}
}

func TestCashierOrderListIsEmployeeScoped(t *testing.T) {
	const secret, issuer, tenant = "order-list-secret", "https://identity.example.test", "order-list-tenant"
	h := New(domain.NewService("http://invalid", "2026-08-07"), config.Config{AppEnv: "dev", APIVersion: "2026-08-07", AuthHMACKey: secret})
	admin := operatorToken(t, secret, "admin", issuer, tenant, "ADMIN")
	type actor struct {
		token    string
		employee domain.Employee
		shiftID  string
		orderID  string
	}
	createActor := func(subject, code, register, suffix string) actor {
		token := operatorToken(t, secret, subject, issuer, tenant, "CASHIER")
		created := operatorRequest(h, http.MethodPost, "/public/v1/minipos/employees", admin, "employee-scope-"+suffix, `{"first_name":"Scoped","last_name":"Cashier","operator_code":"`+code+`"}`)
		if created.Code != http.StatusCreated {
			t.Fatal(created.Code, created.Body.String())
		}
		var employee domain.Employee
		_ = json.Unmarshal(created.Body.Bytes(), &employee)
		bound := operatorRequest(h, http.MethodPost, "/public/v1/minipos/employees/"+employee.ID+"/identity-binding", admin, "binding-scope-"+suffix, `{"subject":"`+subject+`","issuer":"`+issuer+`"}`)
		if bound.Code != http.StatusCreated {
			t.Fatal(bound.Code, bound.Body.String())
		}
		if session := operatorRequest(h, http.MethodGet, "/public/v1/minipos/operator-session", token, "", ""); session.Code != http.StatusOK {
			t.Fatal(session.Code, session.Body.String())
		}
		opened := operatorRequest(h, http.MethodPost, "/public/v1/minipos/shifts", token, "shift-scope-"+suffix, `{"register_id":"`+register+`","employee_id":"`+employee.ID+`"}`)
		if opened.Code != http.StatusCreated {
			t.Fatal(opened.Code, opened.Body.String())
		}
		var shift domain.Shift
		_ = json.Unmarshal(opened.Body.Bytes(), &shift)
		order := operatorRequest(h, http.MethodPost, "/public/v1/minipos/orders", token, "order-scope-"+suffix, `{"shift_id":"`+shift.ID+`"}`)
		if order.Code != http.StatusCreated {
			t.Fatal(order.Code, order.Body.String())
		}
		var createdOrder domain.Order
		_ = json.Unmarshal(order.Body.Bytes(), &createdOrder)
		return actor{token: token, employee: employee, shiftID: shift.ID, orderID: createdOrder.ID}
	}
	first := createActor("cashier-one", "C001", "00000000-0000-4000-8000-000000000001", "0001")
	second := createActor("cashier-two", "C002", "00000000-0000-4000-8000-000000000002", "0002")
	list := operatorRequest(h, http.MethodGet, "/public/v1/minipos/orders", first.token, "", "")
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(first.orderID)) || bytes.Contains(list.Body.Bytes(), []byte(second.orderID)) {
		t.Fatalf("cashier order list leaked another employee: %d %s", list.Code, list.Body.String())
	}
}
