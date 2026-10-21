package domain

import (
	"reflect"
	"testing"
)

func TestNativeOwnedLineMutationCASAndFrozenOrder(t *testing.T) {
	s := NewService("", "")
	const id = "00000000-0000-4000-8000-000000000001"
	const lineID = "00000000-0000-4000-8000-000000000002"
	line := Line{LineID: lineID, Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}
	s.orders[id] = Order{ID: id, TenantID: "tenant", State: "OPEN", Version: 1, Lines: []Line{line}, Total: line.UnitPrice}
	changed := line
	changed.Quantity = "2.000"
	if _, e := s.MutateOrderLineForTenant(id, lineID, 1, &changed, "foreign"); e == nil {
		t.Fatal("foreign tenant accepted")
	}
	if _, e := s.MutateOrderLineForTenant(id, lineID, 0, &changed, "tenant"); e == nil {
		t.Fatal("missing CAS accepted")
	}
	got, e := s.MutateOrderLineForTenant(id, lineID, 1, &changed, "tenant")
	if e != nil || got.Version != 2 || got.Total.Amount != "5.00" {
		t.Fatalf("mutation %+v %v", got, e)
	}
	before := got
	if _, e = s.MutateOrderLineForTenant(id, lineID, 1, nil, "tenant"); e == nil {
		t.Fatal("stale delete accepted")
	}
	if !reflect.DeepEqual(before, s.orders[id]) {
		t.Fatal("failed mutation modified order")
	}
	frozen := before
	frozen.State = "FISCAL_PENDING"
	s.orders[id] = frozen
	if _, e = s.MutateOrderLineForTenant(id, lineID, 2, nil, "tenant"); e == nil {
		t.Fatal("frozen delete accepted")
	}
	s.orders[id] = before
	got, e = s.MutateOrderLineForTenant(id, lineID, 2, nil, "tenant")
	if e != nil || len(got.Lines) != 0 || got.Total.Amount != "0.00" || got.Version != 3 {
		t.Fatalf("delete %+v %v", got, e)
	}
}
