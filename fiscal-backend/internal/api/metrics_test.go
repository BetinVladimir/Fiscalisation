package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpointExposesOperationCounters(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"fiscal_operations_total{state=\"FISCALIZED\"}",
		"fiscal_operations_total{state=\"UNKNOWN\"}",
		"fiscal_unknown_total",
		"mqtt_command_latency_seconds",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q\nbody:\n%s", want, body)
		}
	}
}

func TestLogMaskingHidesEmailAndEGN(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"user email user@example.com order", "user email [REDACTED] order"},
		{"egn 8501010000 registered", "egn [REDACTED] registered"},
		{"no pii here", "no pii here"},
		{"multiple user@test.org and 9912312345", "multiple [REDACTED] and [REDACTED]"},
	}
	for _, tc := range cases {
		got := MaskPII(tc.input)
		if got != tc.want {
			t.Errorf("MaskPII(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
