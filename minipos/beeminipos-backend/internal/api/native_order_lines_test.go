package api

import (
	"bytes"
	"encoding/json"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestNativeSourceLineHTTPAuthorizationCASAndFreeze(t *testing.T) {
	const secret = "native-source-test-secret"
	const issuer = "https://identity.example"
	const tenant = "native-tenant"
	service := domain.NewService("http://unused", "2026-08-07")
	handler := New(service, config.Config{AppEnv: "dev", APIVersion: "2026-08-07", AuthHMACKey: secret})
	admin := operatorToken(t, secret, "admin", issuer, tenant, "ADMIN")
	cashier := operatorToken(t, secret, "cashier", issuer, tenant, "CASHIER")
	foreign := operatorToken(t, secret, "foreign", issuer, "foreign-tenant", "CASHIER")
	created := operatorRequest(handler, "POST", "/public/v1/minipos/employees", admin, "native-employee-0001", `{"first_name":"Native","last_name":"Test","operator_code":"A001"}`)
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var employee domain.Employee
	json.Unmarshal(created.Body.Bytes(), &employee)
	bound := operatorRequest(handler, "POST", "/public/v1/minipos/employees/"+employee.ID+"/identity-binding", admin, "native-identity-0001", `{"subject":"cashier","issuer":"https://identity.example"}`)
	if bound.Code != 201 {
		t.Fatal(bound.Code, bound.Body.String())
	}
	if response := operatorRequest(handler, "GET", "/public/v1/minipos/operator-session", cashier, "", ""); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	shiftResponse := operatorRequest(handler, "POST", "/public/v1/minipos/shifts", cashier, "native-shift-00001", `{"register_id":"00000000-0000-4000-8000-000000000001","employee_id":"`+employee.ID+`"}`)
	if shiftResponse.Code != 201 {
		t.Fatal(shiftResponse.Code, shiftResponse.Body.String())
	}
	var shift domain.Shift
	json.Unmarshal(shiftResponse.Body.Bytes(), &shift)
	orderResponse := operatorRequest(handler, "POST", "/public/v1/minipos/orders", cashier, "native-order-00001", `{"shift_id":"`+shift.ID+`"}`)
	if orderResponse.Code != 201 {
		t.Fatal(orderResponse.Code, orderResponse.Body.String())
	}
	var order domain.Order
	json.Unmarshal(orderResponse.Body.Bytes(), &order)
	const lineID = "00000000-0000-4000-8000-000000000002"
	body := `{"line_id":"` + lineID + `","name":"Coffee","quantity":"1.000","unit_price":{"amount":"2.50","currency":"EUR"},"tax_group":"B"}`
	requestCounter := 0
	request := func(method, path, token, version, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("X-App-Instance-Id", "00000000-0000-4000-8000-000000000001")
		requestCounter++
		r.Header.Set("Idempotency-Key", fmt.Sprintf("native-line-test-%04d", requestCounter))
		if version != "" {
			r.Header.Set("If-Match", version)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/public/v1/minipos/orders/" + order.ID + "/lines"
	if r := request("POST", path, cashier, "1", body); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	path += "/" + lineID
	for _, c := range []struct {
		token, version string
		want           int
	}{{foreign, "2", 404}, {cashier, "", 400}, {cashier, "1", 409}} {
		if r := request(http.MethodPatch, path, c.token, c.version, body); r.Code != c.want {
			t.Fatalf("negative want%d got%d %s", c.want, r.Code, r.Body.String())
		}
	}
	discount := bytes.Replace([]byte(body), []byte(`"tax_group":"B"`), []byte(`"tax_group":"B","discount":{"amount":"0.10","currency":"EUR"}`), 1)
	if r := request("PATCH", path, cashier, "2", string(discount)); r.Code != 403 {
		t.Fatal("cashier discount accepted", r.Code, r.Body.String())
	}
	if r := request("PATCH", path, cashier, "2", body); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if _, e := service.FreezeExecutionOrder(order.ID, shift.ID, employee.ID, tenant, []map[string]any{{"payment_id": "00000000-0000-4000-8000-000000000003", "type": "CASH", "amount": map[string]any{"amount": "2.50", "currency": "EUR"}}}); e != nil {
		t.Fatal(e)
	}
	current, _ := service.OrderForTenant(order.ID, tenant)
	r := request("DELETE", path, cashier, strconv.FormatInt(current.Version, 10), "")
	if r.Code != 409 {
		t.Fatalf("frozen order delete accepted %d %s current%+v", r.Code, r.Body.String(), current)
	}
}
