package runtime

import (
	"crypto/sha256"
	"encoding/json"
	"fiscalisation/edge-agent/ble"
	"testing"
	"time"
)

func TestCanonicalBLEVerticalToDurableFiscalResult(t *testing.T) {
	now := time.Now().UTC()
	payload := map[string]any{"currency": "EUR", "items": []any{map[string]any{"name": "Coffee", "quantity": "1.000"}}, "payments": []any{map[string]any{"type": "CASH", "amount": "2.50"}}, "metadata": map[string]any{"source": "MiniPOS"}}
	hash, err := ble.PayloadHash(payload)
	if err != nil {
		t.Fatal(err)
	}
	envelope := ble.DeviceCommandEnvelope{OperationID: "cmd-ble-1", TenantID: "tenant-1", RegisterID: "r1", DeviceID: "device-1", FencingToken: 7, CommandType: "FISCAL_SALE", IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(time.Minute).Format(time.RFC3339Nano), Payload: payload, PayloadSHA256: hash}
	plain, err := ble.CanonicalMarshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	client, _ := ble.NewEndpoint([]byte("pairing-secret"), "session-1", "client")
	edge, _ := ble.NewEndpoint([]byte("pairing-secret"), "session-1", "edge")
	chunks, _ := ble.ChunkPlaintext(plain, 185)
	reassembler := ble.NewReassembler(1024)
	var messageID [16]byte
	copy(messageID[:], sha256.New().Sum([]byte("message-1")))
	var complete []byte
	for i, chunk := range chunks {
		raw, err := client.SealFrame(messageID, uint16(i), uint16(len(chunks)), 0, chunk)
		if err != nil {
			t.Fatal(err)
		}
		frame, part, err := edge.OpenFrame(raw)
		if err != nil {
			t.Fatal(err)
		}
		_, complete, err = reassembler.Add(frame, part)
		if err != nil {
			t.Fatal(err)
		}
	}
	var decoded ble.DeviceCommandEnvelope
	if err = ble.StrictUnmarshal(complete, &decoded); err != nil {
		t.Fatal(err)
	}
	if err = decoded.Validate(now); err != nil {
		t.Fatal(err)
	}
	rawPayload, _ := json.Marshal(decoded.Payload)
	device := &Simulator{Reachable: true}
	rt, j := setup(t, device)
	defer j.Close()
	result, err := rt.Execute(Command{CommandID: decoded.OperationID, TenantID: decoded.TenantID, RegisterID: decoded.RegisterID, DeviceID: decoded.DeviceID, Type: decoded.CommandType, FencingToken: decoded.FencingToken, Payload: rawPayload})
	if err != nil || result.State != "FISCALIZED" || device.Executions != 1 {
		t.Fatal(result, err)
	}
	again, err := rt.Execute(Command{CommandID: decoded.OperationID, TenantID: decoded.TenantID, RegisterID: decoded.RegisterID, DeviceID: decoded.DeviceID, Type: decoded.CommandType, FencingToken: decoded.FencingToken, Payload: rawPayload})
	if err != nil || again.FiscalReference != result.FiscalReference || device.Executions != 1 {
		t.Fatal("BLE replay executed device twice", again, err)
	}
}
