package sync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fiscalisation/edge-agent/journal"
	"path/filepath"
	"testing"
	"time"
)

func sign(a Ack, key []byte) Ack {
	b := a
	a.Signature = ""
	raw, _ := json.Marshal(a)
	m := hmac.New(sha256.New, key)
	m.Write(raw)
	b.Signature = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	return b
}
func TestOnlySignedContiguousAckMarksJournal(t *testing.T) {
	j, e := journal.Open(filepath.Join(t.TempDir(), "j.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	_, _ = j.Append("op1", "DONE", map[string]int{"n": 1})
	key := []byte("01234567890123456789012345678901")
	v := NewVerifier("edge1", key, j)
	now := time.Now().UTC()
	h := "hash-1"
	if e = v.Expect(Batch{EdgeID: "edge1", FirstSeq: 1, LastSeq: 1, Events: []DeviceEventEnvelope{{EventHash: h}}}); e != nil {
		t.Fatal(e)
	}
	a := sign(Ack{AckID: "a1", EdgeID: "edge1", CommittedThroughSeq: 1, CommittedEventHash: "hash-1", CommittedAt: now, OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)
	bad := a
	bad.Signature = "bad"
	if v.Apply(bad) == nil {
		t.Fatal("bad signature")
	}
	if j.Events()[0].AcknowledgedAt != nil {
		t.Fatal("bad ack applied")
	}
	if e = v.Apply(a); e != nil {
		t.Fatal(e)
	}
	if j.Events()[0].AcknowledgedAt == nil {
		t.Fatal("ack not applied")
	}
	if e = v.Apply(sign(Ack{AckID: "a2", EdgeID: "edge1", CommittedThroughSeq: 1, CommittedEventHash: "hash-1", CommittedAt: now, OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)); e == nil {
		t.Fatal("rollback/replay accepted")
	}
	_, _ = j.Append("op2", "DONE", map[string]int{"n": 2})
	restarted := NewVerifier("edge1", key, j)
	if e = restarted.Expect(Batch{EdgeID: "edge1", FirstSeq: 2, LastSeq: 2, Events: []DeviceEventEnvelope{{EventHash: "hash-2"}}}); e != nil {
		t.Fatal("committed cursor did not survive verifier restart", e)
	}
}
