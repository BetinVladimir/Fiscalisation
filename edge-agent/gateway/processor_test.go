package gateway

import (
	"crypto/sha256"
	"testing"
	"time"

	"fiscalisation/edge-agent/ble"
	edgeruntime "fiscalisation/edge-agent/runtime"
)

func TestEncryptedCommandProducesEncryptedCorrelatedResult(t *testing.T) {
	client, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "client")
	edge, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "edge")
	p, err := NewProcessor(edge, SessionBinding{TenantID: "tenant", RegisterID: "register", DeviceID: "device", FencingToken: 7}, func(c edgeruntime.Command) (edgeruntime.Result, error) {
		if c.TenantID != "tenant" || c.DeviceID != "device" {
			t.Fatal("binding lost")
		}
		return edgeruntime.Result{CommandID: c.CommandID, State: "FISCALIZED", FiscalReference: "FD-42", OperationSequence: 8, UNPSequence: 9}, nil
	}, 185)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	payload := map[string]any{"currency": "EUR", "external_id": "order", "operator_id": "A001", "items": []any{map[string]any{"name": "Coffee"}}, "payments": []any{map[string]any{"type": "CASH"}}, "metadata": map[string]any{}}
	hash, _ := ble.PayloadHash(payload)
	envelope := ble.DeviceCommandEnvelope{OperationID: "operation-1", TenantID: "tenant", RegisterID: "register", DeviceID: "device", FencingToken: 7, CommandType: "FISCAL_SALE", IssuedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(time.Minute).Format(time.RFC3339), Payload: payload, PayloadSHA256: hash}
	encoded, _ := ble.CanonicalMarshal(envelope)
	chunks, _ := ble.ChunkPlaintext(encoded, 185)
	messageID := sha256.Sum256([]byte("operation-1"))
	var id [16]byte
	copy(id[:], messageID[:16])
	var response [][]byte
	for i, chunk := range chunks {
		raw, _ := client.SealFrame(id, uint16(i), uint16(len(chunks)), 0, chunk)
		accepted, acceptErr := p.AcceptCommandFrame(raw)
		if acceptErr != nil {
			t.Fatal(acceptErr)
		}
		if i < len(chunks)-1 && len(accepted.ResponseFrames) != 0 {
			t.Fatal("result emitted before complete command")
		}
		response = accepted.ResponseFrames
	}
	assembler := ble.NewReassembler(1024)
	var complete []byte
	for _, raw := range response {
		frame, plain, openErr := client.OpenFrame(raw)
		if openErr != nil {
			t.Fatal(openErr)
		}
		if frame.Flags != 1 || frame.MessageID != id {
			t.Fatal("result correlation lost")
		}
		_, complete, err = assembler.Add(frame, plain)
		if err != nil {
			t.Fatal(err)
		}
	}
	var result map[string]any
	if err = ble.StrictUnmarshal(complete, &result); err != nil {
		t.Fatal(err)
	}
	if result["state"] != "FISCALIZED" || result["fiscal_reference"] != "FD-42" || result["operation_id"] != "operation-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestProcessorRejectsExpiredEnvelopeBeforeExecution(t *testing.T) {
	client, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "client")
	edge, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "edge")
	calls := 0
	p, _ := NewProcessor(edge, SessionBinding{TenantID: "t", RegisterID: "r", DeviceID: "d", FencingToken: 1}, func(c edgeruntime.Command) (edgeruntime.Result, error) { calls++; return edgeruntime.Result{}, nil }, 185)
	now := time.Now().UTC()
	payload := map[string]any{"currency": "EUR"}
	hash, _ := ble.PayloadHash(payload)
	envelope := ble.DeviceCommandEnvelope{OperationID: "op", TenantID: "t", RegisterID: "r", DeviceID: "d", FencingToken: 1, CommandType: "FISCAL_SALE", IssuedAt: now.Add(-2 * time.Minute).Format(time.RFC3339Nano), ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Payload: payload, PayloadSHA256: hash}
	encoded, _ := ble.CanonicalMarshal(envelope)
	id := [16]byte{1}
	raw, _ := client.SealFrame(id, 0, 1, 0, encoded)
	if _, err := p.AcceptCommandFrame(raw); err == nil || calls != 0 {
		t.Fatal("expired command reached device")
	}
}

func TestDeviceProbeDoesNotExecuteFiscalCommand(t *testing.T) {
	client, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "client")
	edge, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "edge")
	calls := 0
	p, _ := NewProcessor(edge, SessionBinding{TenantID: "t", RegisterID: "r", DeviceID: "d", FencingToken: 1}, func(c edgeruntime.Command) (edgeruntime.Result, error) { calls++; return edgeruntime.Result{}, nil }, 185)
	p.SetFinalDeviceProbe(func() error { return nil })
	now := time.Now().UTC()
	p.now = func() time.Time { return now }
	payload := map[string]any{}
	hash, _ := ble.PayloadHash(payload)
	envelope := ble.DeviceCommandEnvelope{OperationID: "probe-op", TenantID: "t", RegisterID: "r", DeviceID: "d", FencingToken: 1, CommandType: "DEVICE_PROBE", IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(time.Minute).Format(time.RFC3339Nano), Payload: payload, PayloadSHA256: hash}
	encoded, _ := ble.CanonicalMarshal(envelope)
	id := [16]byte{2}
	raw, _ := client.SealFrame(id, 0, 1, 0, encoded)
	accepted, err := p.AcceptCommandFrame(raw)
	if err != nil || calls != 0 {
		t.Fatalf("probe executed fiscal command: %d %v", calls, err)
	}
	frame, plain, err := client.OpenFrame(accepted.ResponseFrames[0])
	if err != nil {
		t.Fatal(err)
	}
	_, complete, err := ble.NewReassembler(10).Add(frame, plain)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if ble.StrictUnmarshal(complete, &result) != nil || result["state"] != "READY" {
		t.Fatalf("bad probe result: %#v", result)
	}
}

func TestSessionBindingMismatchNeverReachesRuntime(t *testing.T) {
	client, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "client")
	edge, _ := ble.NewEndpoint([]byte("session-key-material"), "session", "edge")
	calls := 0
	p, _ := NewProcessor(edge, SessionBinding{TenantID: "tenant-a", RegisterID: "r", DeviceID: "d", FencingToken: 4}, func(c edgeruntime.Command) (edgeruntime.Result, error) { calls++; return edgeruntime.Result{}, nil }, 185)
	now := time.Now().UTC()
	p.now = func() time.Time { return now }
	payload := map[string]any{}
	hash, _ := ble.PayloadHash(payload)
	envelope := ble.DeviceCommandEnvelope{OperationID: "op", TenantID: "tenant-b", RegisterID: "r", DeviceID: "d", FencingToken: 4, CommandType: "FISCAL_SALE", IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(time.Minute).Format(time.RFC3339Nano), Payload: payload, PayloadSHA256: hash}
	encoded, _ := ble.CanonicalMarshal(envelope)
	raw, _ := client.SealFrame([16]byte{3}, 0, 1, 0, encoded)
	if _, err := p.AcceptCommandFrame(raw); err == nil || calls != 0 {
		t.Fatal("cross-tenant BLE command accepted")
	}
}
