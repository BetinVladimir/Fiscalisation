package domain

import (
	"testing"
	"time"
)

func sofiaTime(t *testing.T, value string) time.Time {
	t.Helper()
	location, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatal(err)
	}
	v, err := time.ParseInLocation("2006-01-02 15:04:05", value, location)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOperatorErrorReversalDeadlineUsesSofiaCalendar(t *testing.T) {
	original := sofiaTime(t, "2026-01-31 23:59:00")
	if !reversalAllowed("OPERATOR_ERROR", original, sofiaTime(t, "2026-02-07 23:59:59")) {
		t.Fatal("seventh day was rejected")
	}
	if reversalAllowed("OPERATOR_ERROR", original, sofiaTime(t, "2026-02-08 00:00:00")) {
		t.Fatal("expired operator-error reversal was accepted")
	}
}

func TestOperatorErrorDeadlineHandlesYearAndDSTBoundaries(t *testing.T) {
	if !reversalAllowed("OPERATOR_ERROR", sofiaTime(t, "2025-12-01 00:00:00"), sofiaTime(t, "2026-01-07 23:59:59")) {
		t.Fatal("year boundary was rejected")
	}
	if reversalAllowed("OPERATOR_ERROR", sofiaTime(t, "2026-03-29 03:30:00"), sofiaTime(t, "2026-04-08 00:00:00")) {
		t.Fatal("DST month deadline was extended")
	}
}

func TestReversalReasonsFailClosed(t *testing.T) {
	original := sofiaTime(t, "2026-01-01 12:00:00")
	now := sofiaTime(t, "2026-08-01 12:00:00")
	for _, reason := range []string{"CUSTOMER_RETURN", "CUSTOMER_COMPLAINT", "TAX_BASE_REDUCTION"} {
		if !reversalAllowed(reason, original, now) {
			t.Fatalf("documented reason rejected: %s", reason)
		}
	}
	for _, reason := range []string{"", "OTHER", " customer_return "} {
		if reversalAllowed(reason, original, now) {
			t.Fatalf("undocumented reason accepted: %q", reason)
		}
	}
	if reversalAllowed("CUSTOMER_RETURN", now, original) {
		t.Fatal("reversal before the original operation was accepted")
	}
}
