package domain

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCheckoutPublicAPIAndReplay(t *testing.T) {
	var sales atomic.Int32
	s := NewService("http://fiscal.test", "2026-08-07")
	s.SetFiscalAuthToken("service-token")
	s.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer service-token" {
			t.Fatalf("missing fiscal auth on %s", r.URL.Path)
		}
		body := "{}"
		switch {
		case r.URL.Path == "/sales":
			sales.Add(1)
			body = `{"sale_id":"f-sale-1"}`
		case strings.HasSuffix(r.URL.Path, "/lines"):
			body = `{"sale_id":"f-sale-1"}`
		case strings.HasSuffix(r.URL.Path, "/payments"):
			body = `{"operation_id":"f-op-1","state":"FISCALIZED"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShiftForTenant("FD000001", emp.ID, "tenant-1")
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000001", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	got, e := s.Checkout(o.ID, "1234567890123456", map[string]any{"type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}})
	if e != nil || got.State != "COMPLETED" {
		t.Fatal(got, e)
	}
	payment := map[string]any{"type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}}
	again, e := s.Checkout(o.ID, "1234567890123456", payment)
	if e != nil || again.FiscalOperationID != "f-op-1" || sales.Load() != 1 {
		t.Fatal(again, e, sales.Load())
	}
	if e = s.ApplyFiscalEvent("f-sale-1", "REVERSED", "f-op-reverse", 2); e != nil {
		t.Fatal(e)
	}
	updated, _ := s.Order(o.ID)
	if updated.State != "REVERSED" || updated.FiscalVersion != 2 {
		t.Fatalf("canonical webhook version not applied: %+v", updated)
	}
	if e = s.ApplyFiscalEvent("f-sale-1", "FISCALIZED", "stale", 1); e != nil {
		t.Fatal(e)
	}
	updated, _ = s.Order(o.ID)
	if updated.State != "REVERSED" {
		t.Fatal("stale webhook changed order")
	}
}

func TestOfflineFiscalWebhookLinksOrderByExternalID(t *testing.T) {
	s := NewService("http://fiscal.test", "2026-08-07")
	emp, _ := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShiftForTenant("FD000001", emp.ID, "tenant-1")
	order, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	raw := []byte(`{"event_id":"offline-event","event_type":"fiscal.operation.succeeded","api_version":"2026-08-07","tenant_id":"` + order.TenantID + `","resource_id":"edge-sale-op-1","resource_version":9,"data":{"state":"FISCALIZED","operation_id":"op-1","external_id":"` + order.ExternalID + `"}}`)
	if err := s.ProcessFiscalWebhookLinked("offline-event", raw, "edge-sale-op-1", order.ExternalID, "FISCALIZED", "op-1", 9); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.Order(order.ID)
	if updated.State != "COMPLETED" || updated.FiscalSaleID != "edge-sale-op-1" || updated.FiscalOperationID != "op-1" {
		t.Fatalf("offline order not linked: %+v", updated)
	}
}

func TestFiscalWebhookCannotCrossTenantBoundary(t *testing.T) {
	s := NewService("http://fiscal.test", "2026-08-07")
	s.orders["order-a"] = Order{ID: "order-a", TenantID: "tenant-a", ExternalID: "external-a", FiscalSaleID: "sale-a", State: "UNKNOWN", Version: 1}
	raw := []byte(`{"event_id":"cross-event","event_type":"fiscal.operation.succeeded","api_version":"2026-08-07","tenant_id":"tenant-b","resource_id":"sale-a","resource_version":2,"data":{"state":"FISCALIZED","operation_id":"op-b","external_id":"external-a"}}`)
	if err := s.ProcessFiscalWebhookLinkedForTenant("tenant-b", "cross-event", raw, "sale-a", "external-a", "FISCALIZED", "op-b", 2); err == nil {
		t.Fatal("cross-tenant webhook linked a foreign order")
	}
	order, _ := s.Order("order-a")
	if order.State != "UNKNOWN" || order.FiscalOperationID != "" {
		t.Fatal("cross-tenant webhook mutated order", order)
	}
}

func TestCheckoutRejectsIdempotencyPayloadMismatch(t *testing.T) {
	s := NewService("http://fiscal.test", "2026-08-07")
	s.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"sale_id":"f-sale-1"}`
		if strings.HasSuffix(r.URL.Path, "/payments") {
			body = `{"operation_id":"f-op-1","state":"FISCALIZED"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShift("FD000001", emp.ID)
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000002", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if _, err := s.Checkout(o.ID, "same-key", map[string]any{"type": "CASH"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Checkout(o.ID, "same-key", map[string]any{"type": "CARD"}); err == nil {
		t.Fatal("payload mismatch accepted")
	}
}

func TestCheckoutMalformedFiscalResponseDoesNotPanic(t *testing.T) {
	s := NewService("http://fiscal.test", "2026-08-07")
	s.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"unexpected":true}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShift("FD000001", emp.ID)
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000003", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	got, err := s.Checkout(o.ID, "malformed-response", map[string]any{"type": "CASH"})
	if err == nil || got.State != "UNKNOWN" {
		t.Fatalf("expected safe UNKNOWN outcome, got state=%s err=%v", got.State, err)
	}
}
