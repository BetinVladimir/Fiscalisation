package domain

import (
	"testing"
	"time"
)

// This contract starts as a lost cloud response, continues through the direct
// BLE journal representation and finishes through the same MQTT sync/business
// ACK path used by both Android and ESP-IDF adapters. The profile is routing
// metadata only: operation, receipt and payment identities never change.
func TestThreeProfilesContinueOneOperationAcrossRESTBLEAndMQTT(t *testing.T) {
	for _, profile := range []string{"DATECS_BLUECASH50_EMBEDDED", "DATECS_DP150_BLUEPAD50", "DAISY_COMPACT_S01"} {
		t.Run(profile, func(t *testing.T) {
			key := []byte("01234567890123456789012345678901")
			store := &testStore{}
			repo, _ := NewPersistentRepository(store)
			svc := NewService(repo, NewSimulator(true))
			svc.SetBLESigningKey(string(key))
			opID, receiptID, paymentID := "10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002", "10000000-0000-4000-8000-000000000003"
			accepted := DeviceEventEnvelope{EventID: "accepted-" + profile, OperationID: opID, DeviceID: "device-1", EventType: "ACCEPTED", OccurredAt: "2026-08-07T10:00:01Z", Payload: map[string]any{"operation_sequence": float64(10), "unp_sequence": float64(11), "command": map[string]any{"operation_id": opID, "client_operation_id": opID, "receipt_session_id": receiptID, "tenant_id": "tenant-1", "register_id": "register-1", "device_id": "device-1", "command_type": "FISCAL_SALE", "profile": profile, "payload": map[string]any{"currency": "EUR", "external_id": "order-" + profile, "operator_id": "A001", "items": []any{map[string]any{"line_id": "line-1", "name": "Coffee", "quantity": "1.000", "unit_price": map[string]any{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"}}, "payments": []any{map[string]any{"payment_id": paymentID, "type": "CASH", "amount": map[string]any{"amount": "2.50", "currency": "EUR"}}}}}}}
			ack1, err := svc.SyncBatchForTenant("tenant-1", signCustomBatch("edge-1", 1, nil, []DeviceEventEnvelope{accepted}, key))
			if err != nil {
				t.Fatal(err)
			}
			// Lost business ACK: restart and replay the identical BLE journal event.
			repo, _ = NewPersistentRepository(store)
			svc = NewService(repo, NewSimulator(true))
			svc.SetBLESigningKey(string(key))
			if replay, e := svc.SyncBatchForTenant("tenant-1", signCustomBatch("edge-1", 1, nil, []DeviceEventEnvelope{accepted}, key)); e != nil || replay.CommittedThroughSeq != ack1.CommittedThroughSeq {
				t.Fatal(replay, e)
			}
			previous := ack1.CommittedEventHash
			result := DeviceEventEnvelope{EventID: "result-" + profile, OperationID: opID, DeviceID: "device-1", EventType: "FISCALIZED", OccurredAt: "2026-08-07T10:00:02Z", Payload: map[string]any{"state": "FISCALIZED", "fiscal_reference": "FD-" + profile, "receipt_session_id": receiptID, "payment_id": paymentID}}
			ack2, err := svc.SyncBatchForTenant("tenant-1", signCustomBatch("edge-1", 2, &previous, []DeviceEventEnvelope{result}, key))
			if err != nil || ack2.CommittedThroughSeq != 2 {
				t.Fatal(ack2, err)
			}
			op, err := svc.GetOperation(opID)
			if err != nil || op.State != "FISCALIZED" || op.ID != opID {
				t.Fatal(op, err)
			}
			if pending := svc.PendingOutbox(time.Now().UTC()); len(pending) != 1 {
				t.Fatalf("webhook was not deduplicated: %#v", pending)
			}
		})
	}
}
