package fiscalintegration

import (
	"errors"
	"testing"
)

func TestWebhookNonTerminalStatusKeepsAccepted(t *testing.T) {
	for _, status := range []string{"ACCEPTED", "IN_PROGRESS", "RETRYING"} {
		got, err := mapWebhookStatus(status)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", status, err)
		}
		if got != "ACCEPTED" {
			t.Fatalf("%s: expected ACCEPTED, got %q", status, got)
		}
	}
}

func TestWebhookTerminalStatuses(t *testing.T) {
	cases := map[string]string{
		"SUCCEEDED": "SUCCEEDED",
		"SUPERSEDED": "SUPERSEDED",
		"FAILED":    "FAILED",
		"REJECTED":  "FAILED",
	}
	for input, want := range cases {
		got, err := mapWebhookStatus(input)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", input, err)
		}
		if got != want {
			t.Fatalf("%s: expected %s, got %q", input, want, got)
		}
	}
}

func TestWebhookUnknownStatusRejected(t *testing.T) {
	_, err := mapWebhookStatus("FLYING_PIGS")
	var unknown *ErrUnknownWebhookStatus
	if !errors.As(err, &unknown) {
		t.Fatalf("expected ErrUnknownWebhookStatus, got %v", err)
	}
}
