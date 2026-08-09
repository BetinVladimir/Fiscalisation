package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func signedBatch(edge string, first, last int64, previous *string, key []byte) EdgeSyncBatch {
	events := make([]DeviceEventEnvelope, 0, last-first+1)
	prev := ""
	if previous != nil {
		prev = *previous
	}
	for seq := first; seq <= last; seq++ {
		p := prev
		e := DeviceEventEnvelope{EventID: "event-" + pad7(seq), OperationID: "operation-1", DeviceID: "device-1", JournalSeq: seq, EventType: "FISCALIZED", OccurredAt: time.Date(2026, 8, 7, 10, 0, int(seq), 0, time.UTC).Format(time.RFC3339Nano), Payload: map[string]any{"state": "FISCALIZED"}}
		if p != "" {
			e.PrevHash = &p
		}
		e.EventHash = DeviceEventHash(e)
		prev = e.EventHash
		events = append(events, e)
	}
	v := EdgeSyncBatch{EdgeID: edge, SchemaVersion: "2026-08-07", FirstSeq: first, LastSeq: last, PreviousAcknowledgedHash: previous, Events: events}
	v.BatchSHA256 = EdgeBatchHash(v)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(v.BatchSHA256))
	v.Signature = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	return v
}
func signCustomBatch(edge string, first int64, previous *string, events []DeviceEventEnvelope, key []byte) EdgeSyncBatch {
	prev := ""
	if previous != nil {
		prev = *previous
	}
	for i := range events {
		events[i].JournalSeq = first + int64(i)
		if prev != "" {
			p := prev
			events[i].PrevHash = &p
		}
		events[i].EventHash = DeviceEventHash(events[i])
		prev = events[i].EventHash
	}
	v := EdgeSyncBatch{EdgeID: edge, SchemaVersion: "2026-08-07", FirstSeq: first, LastSeq: first + int64(len(events)) - 1, PreviousAcknowledgedHash: previous, Events: events}
	v.BatchSHA256 = EdgeBatchHash(v)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(v.BatchSHA256))
	v.Signature = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	return v
}

func TestSignedContiguousSyncBatchSurvivesRestart(t *testing.T) {
	store := &testStore{}
	r, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("01234567890123456789012345678901")
	s := NewService(r, NewSimulator(true))
	s.SetBLESigningKey(string(key))
	a, err := s.SyncBatch(signedBatch("edge1", 1, 3, nil, key))
	if err != nil || a.Signature == "" || a.CommittedThroughSeq != 3 || a.CommittedEventHash == "" || len(a.OperationResults) != 1 {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err = base64.RawURLEncoding.DecodeString(a.Signature); err != nil {
		t.Fatal(err)
	}
	r, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s = NewService(r, NewSimulator(true))
	s.SetBLESigningKey(string(key))
	prev := a.CommittedEventHash
	if _, err = s.SyncBatch(signedBatch("edge1", 5, 6, &prev, key)); err == nil {
		t.Fatal("gap accepted")
	}
	if a, err = s.SyncBatch(signedBatch("edge1", 4, 6, &prev, key)); err != nil || a.CommittedThroughSeq != 6 {
		t.Fatalf("%+v %v", a, err)
	}
}

func TestSyncSequenceIsIsolatedByAuthenticatedTenant(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetBLESigningKey(string(key))
	if _, err := s.SyncBatchForTenant("tenant-a", signedBatch("shared-edge", 1, 1, nil, key)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncBatchForTenant("tenant-b", signedBatch("shared-edge", 1, 1, nil, key)); err != nil {
		t.Fatal("another tenant inherited the first tenant journal cursor", err)
	}
}

func TestSyncRejectsCommandOwnedByAnotherTenant(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetBLESigningKey(string(key))
	event := DeviceEventEnvelope{EventID: "accepted-cross", OperationID: "operation-cross", DeviceID: "device-1", EventType: "ACCEPTED", OccurredAt: "2026-08-07T10:00:01Z", Payload: map[string]any{
		"operation_sequence": float64(1), "unp_sequence": float64(1),
		"command": map[string]any{"command_id": "operation-cross", "tenant_id": "tenant-b", "register_id": "register-1", "device_id": "device-1", "type": "FISCAL_SALE", "payload": map[string]any{}},
	}}
	if _, err := s.SyncBatchForTenant("tenant-a", signCustomBatch("edge-1", 1, nil, []DeviceEventEnvelope{event}, key)); err == nil {
		t.Fatal("cross-tenant Edge command accepted")
	}
}

func TestSyncBatchRejectsTamperingAndBrokenEventChain(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	newService := func() *Service {
		s := NewService(NewMemoryRepository(), NewSimulator(true))
		s.SetBLESigningKey(string(key))
		return s
	}
	batch := signedBatch("edge", 1, 2, nil, key)
	batch.Events[0].Payload["state"] = "FAILED"
	if _, err := newService().SyncBatch(batch); err == nil {
		t.Fatal("tampered event accepted")
	}
	batch = signedBatch("edge", 1, 2, nil, key)
	wrong := "wrong"
	batch.Events[1].PrevHash = &wrong
	batch.Events[1].EventHash = DeviceEventHash(batch.Events[1])
	batch.BatchSHA256 = EdgeBatchHash(batch)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(batch.BatchSHA256))
	batch.Signature = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	if _, err := newService().SyncBatch(batch); err == nil {
		t.Fatal("broken event chain accepted")
	}
	batch = signedBatch("edge", 1, 1, nil, key)
	batch.Signature = "invalid"
	if _, err := newService().SyncBatch(batch); err == nil {
		t.Fatal("bad signature accepted")
	}
}

func TestEdgeBuilderGoldenIsAcceptedByFiscalVerifier(t *testing.T) {
	firstHash := "ffa305286e36038004ba64e012f02a957cef46c4b5de4a0777a1e431b9684c9f"
	events := []DeviceEventEnvelope{
		{EventID: "event-1", OperationID: "operation-1", DeviceID: "device-1", JournalSeq: 1, EventType: "ACCEPTED", OccurredAt: "2026-08-07T10:00:01Z", Payload: map[string]any{"state": "ACCEPTED"}, EventHash: firstHash},
		{EventID: "event-2", OperationID: "operation-1", DeviceID: "device-1", JournalSeq: 2, EventType: "FISCALIZED", OccurredAt: "2026-08-07T10:00:02Z", Payload: map[string]any{"state": "FISCALIZED"}, PrevHash: &firstHash, EventHash: "8c5d38b02b7a6029e0d53ff1b3af5495b0674e230bbdf01c28cdd47356f8fe57"},
	}
	v := EdgeSyncBatch{EdgeID: "edge-1", SchemaVersion: "2026-08-07", FirstSeq: 1, LastSeq: 2, Events: events, BatchSHA256: "4d222514b8033d060063c74b0e12b6f334b2895d669fa12ecb5b3538a9d4e34b", Signature: "RMDZi1_gCSGK18kuMr3srrPulqDS5iYYeQaazup7ibs"}
	if DeviceEventHash(events[0]) != events[0].EventHash || DeviceEventHash(events[1]) != events[1].EventHash || EdgeBatchHash(v) != v.BatchSHA256 {
		t.Fatal("Edge/Fiscal sync golden algorithm mismatch")
	}
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetBLESigningKey("01234567890123456789012345678901")
	ack, err := s.SyncBatch(v)
	if err != nil || ack.CommittedThroughSeq != 2 || ack.CommittedEventHash != events[1].EventHash {
		t.Fatalf("golden rejected: %#v %v", ack, err)
	}
}

func TestOfflineFiscalSaleMaterializesAcrossSeparateSyncBatches(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	store := &testStore{}
	repo, _ := NewPersistentRepository(store)
	s := NewService(repo, NewSimulator(true))
	s.SetBLESigningKey(string(key))
	acceptedAt := "2026-08-07T10:00:01Z"
	accepted := DeviceEventEnvelope{EventID: "accepted-1", OperationID: "offline-op-1", DeviceID: "device-1", EventType: "ACCEPTED", OccurredAt: acceptedAt, Payload: map[string]any{
		"operation_sequence": float64(10), "unp_sequence": float64(11),
		"command": map[string]any{"command_id": "offline-op-1", "tenant_id": "tenant-1", "register_id": "register-1", "device_id": "device-1", "type": "FISCAL_SALE", "payload": map[string]any{
			"currency": "EUR", "external_id": "order-1", "operator_id": "A001", "metadata": map[string]any{"source": "MiniPOS"},
			"items":    []any{map[string]any{"line_id": "line-1", "name": "Coffee", "quantity": "1.000", "unit_price": map[string]any{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"}},
			"payments": []any{map[string]any{"payment_id": "payment-1", "type": "CASH", "amount": map[string]any{"amount": "2.50", "currency": "EUR"}}},
		}},
	}}
	ack1, err := s.SyncBatch(signCustomBatch("edge-1", 1, nil, []DeviceEventEnvelope{accepted}, key))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetOperation("offline-op-1"); err == nil {
		t.Fatal("operation materialized before device result")
	}
	repo, _ = NewPersistentRepository(store)
	s = NewService(repo, NewSimulator(true))
	s.SetBLESigningKey(string(key))
	previous := ack1.CommittedEventHash
	result := DeviceEventEnvelope{EventID: "result-1", OperationID: "offline-op-1", DeviceID: "device-1", EventType: "FISCALIZED", OccurredAt: "2026-08-07T10:00:02Z", Payload: map[string]any{"state": "FISCALIZED", "fiscal_reference": "FD-RECEIPT-1"}}
	ack2, err := s.SyncBatch(signCustomBatch("edge-1", 2, &previous, []DeviceEventEnvelope{result}, key))
	if err != nil || ack2.CommittedThroughSeq != 2 {
		t.Fatal(ack2, err)
	}
	op, err := s.GetOperation("offline-op-1")
	if err != nil || op.State != "FISCALIZED" || op.FiscalReference != "FD-RECEIPT-1" {
		t.Fatal(op, err)
	}
	sale, err := s.GetSale("edge-sale-offline-op-1")
	if err != nil || sale.State != "COMPLETED" || sale.UNP != "register-1-A001-0000011" || sale.ReceiptArtifactID == "" {
		t.Fatal(sale, err)
	}
	if _, err = s.Receipt(sale.ID); err != nil {
		t.Fatal("offline receipt unavailable", err)
	}
	if pending := s.PendingOutbox(time.Now().UTC()); len(pending) != 1 || pending[0].Event.EventType != "fiscal.operation.succeeded" {
		t.Fatalf("offline webhook missing: %#v", pending)
	}
}
