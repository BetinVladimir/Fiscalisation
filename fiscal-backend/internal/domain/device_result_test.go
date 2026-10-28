package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeviceResultStatusValues(t *testing.T) {
	if DeviceResultCommitted != "COMMITTED" {
		t.Fatalf("expected COMMITTED, got %q", DeviceResultCommitted)
	}
	if DeviceResultNotCommitted != "NOT_COMMITTED" {
		t.Fatalf("expected NOT_COMMITTED, got %q", DeviceResultNotCommitted)
	}
	if DeviceResultUnknown != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN, got %q", DeviceResultUnknown)
	}
}

// TestDeviceResultUnknownNotFailed is the invariant test: UNKNOWN must not
// serialise as "FAILED" or any other status string. After a write timeout the
// domain records UNKNOWN and must never coerce it to FAILED.
func TestDeviceResultUnknownNotFailed(t *testing.T) {
	r := DeviceResult{Status: DeviceResultUnknown, Evidence: "timeout-ref-42"}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "FAILED") {
		t.Fatalf("UNKNOWN serialised as FAILED: %s", s)
	}
	if !strings.Contains(s, "UNKNOWN") {
		t.Fatalf("UNKNOWN status not present in JSON: %s", s)
	}

	// Round-trip
	var got DeviceResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if got.Status != DeviceResultUnknown {
		t.Fatalf("after round-trip expected UNKNOWN, got %q", got.Status)
	}
	if got.Evidence != "timeout-ref-42" {
		t.Fatalf("Evidence mismatch: %q", got.Evidence)
	}
}

func TestDeviceResultCommittedRoundTrip(t *testing.T) {
	r := DeviceResult{Status: DeviceResultCommitted, Evidence: "doc-001"}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var got DeviceResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if got.Status != DeviceResultCommitted {
		t.Fatalf("expected COMMITTED, got %q", got.Status)
	}
}

func TestDeviceResultNotCommittedRoundTrip(t *testing.T) {
	r := DeviceResult{Status: DeviceResultNotCommitted}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var got DeviceResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if got.Status != DeviceResultNotCommitted {
		t.Fatalf("expected NOT_COMMITTED, got %q", got.Status)
	}
}
