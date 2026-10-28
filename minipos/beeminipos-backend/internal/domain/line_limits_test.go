package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRejectsPriceTimesQuantityOverLimit(t *testing.T) {
	// Unit test of ValidateLine.
	price := Money{Amount: "9999999.99", Currency: "EUR"}
	if err := ValidateLine(price, "2.000"); !errors.Is(err, ErrLineLimitExceeded) {
		t.Fatalf("expected ErrLineLimitExceeded for 9999999.99 × 2, got %v", err)
	}
	price2 := Money{Amount: "1200.00", Currency: "EUR"}
	if err := ValidateLine(price2, "1.000"); err != nil {
		t.Fatalf("expected nil for 1200.00 × 1, got %v", err)
	}

	// Integration: AddLineExpected must also enforce the limit.
	svc := NewService("http://fiscal.test", "test")
	oid := "00000000-0000-4000-8000-000000000010"
	svc.orders[oid] = Order{
		ID: oid, State: "DRAFT", Lines: []Line{},
		Total: Money{Amount: "0.00", Currency: "EUR"}, Version: 1, TenantID: "t1",
		UpdatedAt: time.Now().UTC(), CreatedAt: time.Now().UTC(),
	}
	overLimit := Line{
		LineID:    "00000000-0000-4000-8000-000000000011",
		Name:      "item",
		Quantity:  "2.000",
		UnitPrice: Money{Amount: "9999999.99", Currency: "EUR"},
		TaxGroup:  "A",
	}
	if _, err := svc.AddLineExpected(oid, 0, overLimit); !errors.Is(err, ErrLineLimitExceeded) {
		t.Fatalf("AddLineExpected should reject over-limit line, got %v", err)
	}
}

func TestRejectsNegativeUnitPrice(t *testing.T) {
	// Negative amounts are rejected by the Money format, but ValidateLine must
	// also reject them explicitly so callers do not need to rely on validMoney.
	price := Money{Amount: "-1.00", Currency: "EUR"}
	if err := ValidateLine(price, "1.000"); !errors.Is(err, ErrNegativeUnitPrice) {
		t.Fatalf("expected ErrNegativeUnitPrice for -1.00, got %v", err)
	}
}

func TestTaxGroupSetMatchesFiscal(t *testing.T) {
	// Datecs/Daisy support tax groups A-H only. Verify AddLineExpected enforces this.
	svc := NewService("http://fiscal.test", "test")
	// Inject a minimal DRAFT order.
	oid := "00000000-0000-4000-8000-000000000001"
	svc.orders[oid] = Order{
		ID:       oid,
		State:    "DRAFT",
		Lines:    []Line{},
		Total:    Money{Amount: "0.00", Currency: "EUR"},
		Version:  1,
		TenantID: "t1",
		UpdatedAt: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}
	lineBase := Line{
		LineID:    "00000000-0000-4000-8000-000000000002",
		Name:      "item",
		Quantity:  "1.000",
		UnitPrice: Money{Amount: "12.00", Currency: "EUR"},
	}
	for _, tg := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		l := lineBase
		l.TaxGroup = tg
		if _, err := svc.AddLineExpected(oid, 0, l); err != nil {
			t.Errorf("tax group %q should be accepted, got %v", tg, err)
		}
		// Reset order to DRAFT for next iteration.
		o := svc.orders[oid]
		o.State = "DRAFT"
		o.Lines = []Line{}
		o.Total = Money{Amount: "0.00", Currency: "EUR"}
		svc.orders[oid] = o
	}
	for _, tg := range []string{"I", "Z", "0", ""} {
		l := lineBase
		l.TaxGroup = tg
		if _, err := svc.AddLineExpected(oid, 0, l); err == nil {
			t.Errorf("tax group %q should be rejected", tg)
		}
	}
}
