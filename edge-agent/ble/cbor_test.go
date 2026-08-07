package ble

import (
	"bytes"
	"testing"
	"time"
)

func TestCanonicalCBORCommandEnvelope(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	payloadA := map[string]any{"z": uint64(2), "a": "first"}
	payloadB := map[string]any{"a": "first", "z": uint64(2)}
	a, _ := CanonicalMarshal(payloadA)
	b, _ := CanonicalMarshal(payloadB)
	if !bytes.Equal(a, b) {
		t.Fatal("map order changed deterministic encoding")
	}
	hash, _ := PayloadHash(payloadA)
	v := DeviceCommandEnvelope{OperationID: "op-1", TenantID: "t-1", RegisterID: "r-1", DeviceID: "d-1", FencingToken: 1, CommandType: "SALE", IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(time.Minute).Format(time.RFC3339Nano), Payload: payloadA, PayloadSHA256: hash}
	raw, err := CanonicalMarshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DeviceCommandEnvelope
	if err = StrictUnmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err = decoded.Validate(now); err != nil {
		t.Fatal(err)
	}
	decoded.PayloadSHA256 = "bad"
	if err = decoded.Validate(now); err == nil {
		t.Fatal("bad payload hash accepted")
	}
	decoded.PayloadSHA256 = hash
	if err = decoded.Validate(now.Add(2 * time.Minute)); err == nil {
		t.Fatal("expired command accepted")
	}
}
