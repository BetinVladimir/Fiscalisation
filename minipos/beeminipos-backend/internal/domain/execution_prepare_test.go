package domain

import (
	"testing"
	"time"
)

func TestExecutionFreezeAndTrustedNamespace(t *testing.T) {
	s := NewService("unused", "v")
	s.shifts["shift"] = Shift{ID: "shift", TenantID: "tenant", EmployeeID: "employee", State: "OPEN"}
	s.orders["order"] = Order{ID: "order", TenantID: "tenant", ShiftID: "shift", RegisterID: "source-register", State: "OPEN", Version: 1, Lines: []Line{{LineID: "line", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}}, Total: Money{Amount: "2.50", Currency: "EUR"}, UpdatedAt: time.Now()}
	payments := []map[string]any{{"payment_id": "20000000-0000-4000-8000-000000000001", "type": "CASH", "amount": map[string]any{"amount": "2.50", "currency": "EUR"}}}
	frozen, e := s.FreezeExecutionOrder("order", "shift", "employee", "tenant", payments)
	if e != nil || frozen.State != "FISCAL_PENDING" {
		t.Fatal(frozen, e)
	}
	if _, e = s.FreezeExecutionOrder("order", "shift", "foreign", "tenant", payments); e == nil {
		t.Fatal("foreign actor")
	}
	again, e := s.FreezeExecutionOrder("order", "shift", "employee", "tenant", payments)
	if e != nil || again.Version != frozen.Version {
		t.Fatal("retry", e)
	}
	if _, e = s.AttachPreparedFiscalExecution("order", "tenant", "real-sale", "10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002", "root-register", "auth"); e != nil {
		t.Fatal(e)
	}
	saved := s.orders["order"]
	if FiscalRegisterForOrder(saved) != "root-register" || saved.RegisterID != "source-register" || saved.FiscalSaleID == saved.ID {
		t.Fatal("namespace conflated", saved)
	}
	if _, e = s.AttachPreparedFiscalExecution("order", "tenant", "other-sale", "10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002", "root-register", "auth"); e == nil {
		t.Fatal("replaced prepared sale")
	}
	payments[0]["type"] = "CARD"
	if _, e = s.FreezeExecutionOrder("order", "shift", "employee", "tenant", payments); e == nil {
		t.Fatal("changed tender")
	}
}
