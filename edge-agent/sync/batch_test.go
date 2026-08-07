package sync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"fiscalisation/edge-agent/journal"
)

func TestBuildBatchCreatesContractHashChainAndSignature(t *testing.T) {
	j, err := journal.Open(filepath.Join(t.TempDir(), "edge.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	_, _ = j.Append("operation-1", "COMMAND_DURABLE", map[string]any{"state": "ACCEPTED"})
	_, _ = j.Append("operation-1", "COMMAND_RESULT", map[string]any{"state": "FISCALIZED"})
	key := []byte("01234567890123456789012345678901")
	v, err := BuildBatch("edge-1", "device-1", j.Events(), nil, key)
	if err != nil || v.FirstSeq != 1 || v.LastSeq != 2 || len(v.Events) != 2 || v.Events[0].EventType != "ACCEPTED" || v.Events[1].EventType != "FISCALIZED" || v.Events[1].PrevHash == nil || *v.Events[1].PrevHash != v.Events[0].EventHash || batchHash(v) != v.BatchSHA256 {
		t.Fatalf("invalid batch: %#v err=%v", v, err)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(v.Signature)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(v.BatchSHA256))
	if !hmac.Equal(sig, m.Sum(nil)) {
		t.Fatal("invalid batch signature")
	}
	v.Events[0].Payload["state"] = "FAILED"
	if batchHash(v) == v.BatchSHA256 {
		t.Fatal("tampering did not change batch hash")
	}
}

func TestDeterministicGoldenMatchesFiscalVerifier(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	events := []journal.Event{{Sequence: 1, EventID: "event-1", OperationID: "operation-1", Type: "COMMAND_DURABLE", Payload: []byte(`{"state":"ACCEPTED"}`), CreatedAt: time.Date(2026, 8, 7, 10, 0, 1, 0, time.UTC)}, {Sequence: 2, EventID: "event-2", OperationID: "operation-1", Type: "COMMAND_RESULT", Payload: []byte(`{"state":"FISCALIZED"}`), CreatedAt: time.Date(2026, 8, 7, 10, 0, 2, 0, time.UTC)}}
	v, err := BuildBatch("edge-1", "device-1", events, nil, key)
	if err != nil {
		t.Fatal(err)
	}
	if v.Events[0].EventHash != "ffa305286e36038004ba64e012f02a957cef46c4b5de4a0777a1e431b9684c9f" || v.Events[1].EventHash != "8c5d38b02b7a6029e0d53ff1b3af5495b0674e230bbdf01c28cdd47356f8fe57" || v.BatchSHA256 != "4d222514b8033d060063c74b0e12b6f334b2895d669fa12ecb5b3538a9d4e34b" || v.Signature != "RMDZi1_gCSGK18kuMr3srrPulqDS5iYYeQaazup7ibs" {
		t.Fatalf("sync golden changed: %#v", v)
	}
}

func TestBuildBatchRejectsJournalGap(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	events := []journal.Event{{Sequence: 1, EventID: "e1", OperationID: "o", Type: "DONE", Payload: []byte(`{"state":"FISCALIZED"}`)}, {Sequence: 3, EventID: "e3", OperationID: "o", Type: "DONE", Payload: []byte(`{"state":"FISCALIZED"}`)}}
	if _, err := BuildBatch("edge", "device", events, nil, key); err == nil {
		t.Fatal("journal gap accepted")
	}
}

func TestBuildNextBatchUsesDurableAcknowledgedCursor(t *testing.T) {
	j, err := journal.Open(filepath.Join(t.TempDir(), "cursor.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	_, _ = j.Append("o1", "DONE", map[string]any{"state": "FISCALIZED"})
	_, _ = j.Append("o2", "DONE", map[string]any{"state": "FISCALIZED"})
	key := []byte("01234567890123456789012345678901")
	first, err := BuildNextBatch(j, "edge", "device", 1, key)
	if err != nil || first.FirstSeq != 1 || first.LastSeq != 1 {
		t.Fatal(first, err)
	}
	if err = j.ApplySyncAcknowledgement("edge", 1, first.Events[0].EventHash, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	second, err := BuildNextBatch(j, "edge", "device", 100, key)
	if err != nil || second.FirstSeq != 2 || second.PreviousAcknowledgedHash == nil || *second.PreviousAcknowledgedHash != first.Events[0].EventHash {
		t.Fatal(second, err)
	}
}
