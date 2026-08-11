package domain

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCheckoutPublicAPIAndReplay(t *testing.T) {
	var sales atomic.Int32
	var sentLine Line
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
			body = `{"sale_id":"f-sale-1","version":1}`
		case strings.HasSuffix(r.URL.Path, "/lines"):
			if err := json.NewDecoder(r.Body).Decode(&sentLine); err != nil {
				t.Fatal(err)
			}
			if r.Header.Get("If-Match") != "1" {
				t.Fatalf("missing fiscal sale version guard: %q", r.Header.Get("If-Match"))
			}
			body = `{"sale_id":"f-sale-1","version":2}`
		case strings.HasSuffix(r.URL.Path, "/payments"):
			body = `{"operation_id":"f-op-1","state":"FISCALIZED","fiscal_reference":"FD-1"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShiftForTenant("00000000-0000-4000-8000-000000000001", emp.ID, "tenant-1")
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	discount := Money{Amount: "0.20", Currency: "EUR"}
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000001", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, Discount: &discount, TaxGroup: "B"})
	payment := map[string]any{"payment_id": "00000000-0000-4000-8000-000000000011", "type": "CASH", "amount": map[string]string{"amount": "2.30", "currency": "EUR"}, "terminal_policy": "NONE"}
	got, e := s.Checkout(o.ID, "1234567890123456", payment)
	if e != nil || got.State != "COMPLETED" {
		t.Fatal(got, e)
	}
	if sentLine.Discount == nil || sentLine.Discount.Amount != "0.20" || sentLine.Discount.Currency != "EUR" {
		t.Fatalf("discount lost on MiniPOS -> Fiscal command: %#v", sentLine)
	}
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
	sh, _ := s.OpenShiftForTenant("00000000-0000-4000-8000-000000000001", emp.ID, "tenant-1")
	order, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	raw := []byte(`{"event_id":"offline-event","event_type":"fiscal.operation.succeeded","api_version":"2026-08-07","tenant_id":"` + order.TenantID + `","resource_id":"edge-sale-op-1","resource_version":9,"data":{"state":"FISCALIZED","operation_id":"op-1","external_id":"` + order.ExternalID + `","fiscal_reference":"FD-OFFLINE-1"}}`)
	if err := s.ProcessFiscalWebhookLinkedForTenantWithReference(order.TenantID, "offline-event", raw, "edge-sale-op-1", order.ExternalID, "FISCALIZED", "op-1", "FD-OFFLINE-1", 9); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.Order(order.ID)
	if updated.State != "COMPLETED" || updated.FiscalSaleID != "edge-sale-op-1" || updated.FiscalOperationID != "op-1" || updated.ReceiptReference != "FD-OFFLINE-1" {
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
		body := `{"sale_id":"f-sale-1","version":1}`
		if strings.HasSuffix(r.URL.Path, "/lines") {
			body = `{"sale_id":"f-sale-1","version":2}`
		}
		if strings.HasSuffix(r.URL.Path, "/payments") {
			body = `{"operation_id":"f-op-1","state":"FISCALIZED","fiscal_reference":"FD-2"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShift("00000000-0000-4000-8000-000000000001", emp.ID)
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000002", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	cash := map[string]any{"payment_id": "00000000-0000-4000-8000-000000000012", "type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}, "terminal_policy": "NONE"}
	card := map[string]any{"payment_id": "00000000-0000-4000-8000-000000000013", "type": "CARD", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}, "terminal_policy": "AUTO_IF_AVAILABLE"}
	if _, err := s.Checkout(o.ID, "same-key", cash); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Checkout(o.ID, "same-key", card); err == nil {
		t.Fatal("payload mismatch accepted")
	}
}

func TestCheckoutMalformedFiscalResponseDoesNotPanic(t *testing.T) {
	s := NewService("http://fiscal.test", "2026-08-07")
	s.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"unexpected":true}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShift("00000000-0000-4000-8000-000000000001", emp.ID)
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000003", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	got, err := s.Checkout(o.ID, "malformed-response", map[string]any{"payment_id": "00000000-0000-4000-8000-000000000014", "type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}, "terminal_policy": "NONE"})
	if err == nil || got.State != "UNKNOWN" {
		t.Fatalf("expected safe UNKNOWN outcome, got state=%s err=%v", got.State, err)
	}
}

func TestCheckoutBatchSplitCashAndCard(t *testing.T) {
	var paymentCalls atomic.Int32
	s := NewService("http://fiscal.test", "2026-08-07")
	s.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"sale_id":"f-sale-split","version":1}`
		if strings.HasSuffix(r.URL.Path, "/lines") {
			body = `{"sale_id":"f-sale-split","version":2}`
		}
		if strings.HasSuffix(r.URL.Path, "/payments") {
			n := paymentCalls.Add(1)
			if n == 1 {
				body = `{"operation_id":"f-op-cash","state":"EXECUTING"}`
			} else {
				body = `{"operation_id":"f-op-card","state":"FISCALIZED","fiscal_reference":"FD-SPLIT-1"}`
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShift("00000000-0000-4000-8000-000000000001", emp.ID)
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000015", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	payments := []map[string]any{
		{"payment_id": "00000000-0000-4000-8000-000000000016", "type": "CASH", "amount": map[string]string{"amount": "1.00", "currency": "EUR"}, "terminal_policy": "NONE"},
		{"payment_id": "00000000-0000-4000-8000-000000000017", "type": "CARD", "amount": map[string]string{"amount": "1.50", "currency": "EUR"}, "terminal_policy": "AUTO_IF_AVAILABLE"},
	}
	got, err := s.CheckoutBatchForTenant(o.ID, "split-checkout-key", payments, nil, o.TenantID)
	if err != nil || got.State != "COMPLETED" || got.FiscalOperationID != "f-op-card" || got.ReceiptReference != "FD-SPLIT-1" || paymentCalls.Load() != 2 {
		t.Fatalf("split checkout failed: order=%+v calls=%d err=%v", got, paymentCalls.Load(), err)
	}
}

func TestCheckoutRejectsOpaqueMixedAndInvalidSplitTotal(t *testing.T) {
	s := NewService("http://fiscal.test", "2026-08-07")
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShift("00000000-0000-4000-8000-000000000001", emp.ID)
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000018", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	mixed := map[string]any{"payment_id": "00000000-0000-4000-8000-000000000019", "type": "MIXED", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}}
	if _, err := s.Checkout(o.ID, "mixed-key", mixed); err == nil {
		t.Fatal("opaque MIXED tender accepted")
	}
	wrong := []map[string]any{
		{"payment_id": "00000000-0000-4000-8000-000000000020", "type": "CASH", "amount": map[string]string{"amount": "1.00", "currency": "EUR"}, "terminal_policy": "NONE"},
		{"payment_id": "00000000-0000-4000-8000-000000000021", "type": "CARD", "amount": map[string]string{"amount": "1.00", "currency": "EUR"}, "terminal_policy": "REQUIRED"},
	}
	if _, err := s.CheckoutBatchForTenant(o.ID, "wrong-total-key", wrong, nil, o.TenantID); err == nil {
		t.Fatal("split total mismatch accepted")
	}
}

func TestCompletedOrderReversalUsesFiscalPublicAPIAndIsIdempotent(t *testing.T) {
	var reversals atomic.Int32
	var webhookWins atomic.Bool
	var s *Service
	s = NewService("http://fiscal.test", "2026-08-07")
	s.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"sale_id":"f-sale-reversal","version":1}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/lines"):
			body = `{"sale_id":"f-sale-reversal","version":2}`
		case strings.HasSuffix(r.URL.Path, "/payments"):
			body = `{"operation_id":"f-op-sale","state":"FISCALIZED","fiscal_reference":"receipt-1","simulated":true}`
		case strings.HasSuffix(r.URL.Path, "/reversals"):
			reversals.Add(1)
			if r.Header.Get("Idempotency-Key") != "reversal-idempotency-key-fiscal-reversal" {
				t.Fatalf("unexpected fiscal reversal key: %s", r.Header.Get("Idempotency-Key"))
			}
			body = `{"operation_id":"f-op-reversal","state":"FISCALIZED","fiscal_reference":"storno-1","simulated":true}`
			if webhookWins.Load() {
				if err := s.applyFiscalEventLinkedForTenant("", "f-sale-reversal", "", "REVERSED", "f-op-reversal", "storno-1", 1); err != nil {
					t.Fatalf("webhook race setup failed: %v", err)
				}
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	sh, _ := s.OpenShift("00000000-0000-4000-8000-000000000001", emp.ID)
	o, _ := s.CreateOrder(Order{ShiftID: sh.ID})
	o, _ = s.AddLine(o.ID, Line{LineID: "00000000-0000-4000-8000-000000000022", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	_, err := s.Checkout(o.ID, "checkout-before-reversal", map[string]any{"payment_id": "00000000-0000-4000-8000-000000000023", "type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}, "terminal_policy": "NONE"})
	if err != nil {
		t.Fatal(err)
	}
	webhookWins.Store(true)
	result, err := s.ReverseOrderForTenant(o.ID, "reversal-idempotency-key", "CUSTOMER_RETURN", o.TenantID)
	if err != nil || result.State != "FISCALIZED" || result.OperationID != "f-op-reversal" || reversals.Load() != 1 {
		t.Fatalf("reversal failed: result=%+v calls=%d err=%v", result, reversals.Load(), err)
	}
	updated, _ := s.Order(o.ID)
	if updated.State != "REVERSED" || updated.ReversalOperationID != "f-op-reversal" || updated.ReversalFiscalReference != "storno-1" || updated.FiscalOperationID != "f-op-sale" {
		t.Fatalf("original/reversal linkage lost: %+v", updated)
	}
	if _, err = s.ReverseOrderForTenant(o.ID, "another-reversal-key", "CUSTOMER_RETURN", o.TenantID); err == nil || reversals.Load() != 1 {
		t.Fatal("second reversal reached Fiscal")
	}
}

func TestMiniPOSExactOrderArithmeticRejectsOverflow(t *testing.T) {
	discount := Money{Amount: "0.20", Currency: "EUR"}
	total, err := orderTotal([]Line{
		{Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, Discount: &discount},
		{Quantity: "1.200", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}},
		{Quantity: "0.500", UnitPrice: Money{Amount: "0.01", Currency: "EUR"}},
	})
	if err != nil || total.Amount != "2.01" || total.Currency != "EUR" {
		t.Fatalf("exact MiniPOS half-up total mismatch: %#v %v", total, err)
	}
	if _, err = orderTotal([]Line{{Quantity: "2.000", UnitPrice: Money{Amount: "92233720368547758.07", Currency: "EUR"}}}); err == nil {
		t.Fatal("overflowing MiniPOS line total accepted")
	}
	over := Money{Amount: "1.01", Currency: "EUR"}
	if _, err = orderTotal([]Line{{Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, Discount: &over}}); err == nil {
		t.Fatal("over-discounted MiniPOS line accepted")
	}
}

func TestConcurrentReversalFreezesOrderBeforeFiscalCall(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var reversals atomic.Int32
	s := NewService("http://fiscal.test", "2026-08-07")
	s.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"sale_id":"f-sale-reversal","version":1}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/lines"):
			body = `{"sale_id":"f-sale-reversal","version":2}`
		case strings.HasSuffix(r.URL.Path, "/payments"):
			body = `{"operation_id":"f-op-sale","state":"FISCALIZED","fiscal_reference":"receipt-1"}`
		case strings.HasSuffix(r.URL.Path, "/reversals"):
			if reversals.Add(1) == 1 {
				close(entered)
				<-release
			}
			body = `{"operation_id":"f-op-reversal","state":"FISCALIZED","fiscal_reference":"storno-1"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})}
	emp, _ := s.CreateEmployee(Employee{FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	shift, _ := s.OpenShift("00000000-0000-4000-8000-000000000001", emp.ID)
	order, _ := s.CreateOrder(Order{ShiftID: shift.ID})
	order, _ = s.AddLine(order.ID, Line{LineID: "00000000-0000-4000-8000-000000000024", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if _, err := s.Checkout(order.ID, "checkout-before-concurrent-reversal", map[string]any{"payment_id": "00000000-0000-4000-8000-000000000025", "type": "CASH", "amount": map[string]string{"amount": "2.50", "currency": "EUR"}, "terminal_policy": "NONE"}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		_, err := s.ReverseOrderForTenant(order.ID, "reversal-one", "CUSTOMER_RETURN", order.TenantID)
		errs <- err
	}()
	<-entered
	go func() {
		defer wg.Done()
		_, err := s.ReverseOrderForTenant(order.ID, "reversal-two", "CUSTOMER_RETURN", order.TenantID)
		errs <- err
	}()
	close(release)
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if reversals.Load() != 1 || successes != 1 {
		t.Fatalf("concurrent reversal emitted %d storno calls with %d successes", reversals.Load(), successes)
	}
}
