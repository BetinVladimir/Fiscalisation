package domain

import (
	"testing"
	"time"
)

func TestSalesReportUsesHalfOpenPeriodAndPaymentEvidence(t *testing.T) {
	s := NewService("http://invalid", "2026-08-07")
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	s.orders["cash"] = Order{ID: "cash", State: "COMPLETED", Total: Money{Amount: "10.00", Currency: "EUR"}, Payments: []OrderPayment{{Type: "CASH", Amount: Money{Amount: "10.00", Currency: "EUR"}}}, CreatedAt: from}
	s.orders["card-reversed"] = Order{ID: "card-reversed", State: "REVERSED", Total: Money{Amount: "2.50", Currency: "EUR"}, Payments: []OrderPayment{{Type: "CARD", Amount: Money{Amount: "2.50", Currency: "EUR"}}}, CreatedAt: from.Add(time.Hour)}
	s.orders["boundary"] = Order{ID: "boundary", State: "COMPLETED", Total: Money{Amount: "99.00", Currency: "EUR"}, Payments: []OrderPayment{{Type: "CASH", Amount: Money{Amount: "99.00", Currency: "EUR"}}}, CreatedAt: to}

	report := s.SalesReportForPeriod("", from, to)
	if report["gross"].(Money).Amount != "7.50" {
		t.Fatalf("unexpected net gross: %#v", report)
	}
	payments := report["payments"].([]map[string]any)
	if len(payments) != 2 || payments[0]["type"] != "CARD" || payments[0]["amount"].(Money).Amount != "-2.50" || payments[1]["type"] != "CASH" || payments[1]["amount"].(Money).Amount != "10.00" {
		t.Fatalf("unexpected payment breakdown: %#v", payments)
	}
}
