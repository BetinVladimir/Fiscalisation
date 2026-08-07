package sync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fiscalisation/edge-agent/journal"
	"sync"
	"time"
)

type Ack struct {
	AckID               string            `json:"ack_id"`
	EdgeID              string            `json:"edge_id"`
	CommittedThroughSeq int64             `json:"committed_through_seq"`
	CommittedEventHash  string            `json:"committed_event_hash"`
	CommittedAt         time.Time         `json:"committed_at"`
	OperationResults    []OperationResult `json:"operation_results"`
	Rejected            []map[string]any  `json:"rejected"`
	Signature           string            `json:"signature"`
}
type OperationResult struct {
	OperationID string `json:"operation_id"`
	State       string `json:"state"`
	Version     int64  `json:"version"`
}
type Verifier struct {
	mu           sync.Mutex
	edgeID       string
	key          []byte
	committed    int64
	expectedSeq  int64
	expectedHash string
	journal      *journal.Journal
}

func NewVerifier(edge string, key []byte, j *journal.Journal) *Verifier {
	v := &Verifier{edgeID: edge, key: key, journal: j}
	if through, _, ok := j.SyncState(edge); ok {
		v.committed = through
	}
	return v
}
func (v *Verifier) Expect(batch Batch) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if batch.EdgeID != v.edgeID || batch.FirstSeq != v.committed+1 || batch.LastSeq < batch.FirstSeq || len(batch.Events) == 0 || batch.Events[len(batch.Events)-1].EventHash == "" {
		return errors.New("invalid pending batch")
	}
	v.expectedSeq, v.expectedHash = batch.LastSeq, batch.Events[len(batch.Events)-1].EventHash
	return nil
}
func (v *Verifier) Apply(a Ack) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if a.EdgeID != v.edgeID || v.expectedSeq == 0 || a.CommittedThroughSeq != v.expectedSeq || a.CommittedEventHash != v.expectedHash {
		return errors.New("non-contiguous ack")
	}
	sig, e := base64.RawURLEncoding.DecodeString(a.Signature)
	if e != nil {
		return e
	}
	unsigned := a
	unsigned.Signature = ""
	b, _ := json.Marshal(unsigned)
	m := hmac.New(sha256.New, v.key)
	m.Write(b)
	if !hmac.Equal(sig, m.Sum(nil)) {
		return errors.New("invalid ack signature")
	}
	if e = v.journal.ApplySyncAcknowledgement(v.edgeID, a.CommittedThroughSeq, a.CommittedEventHash, a.CommittedAt); e != nil {
		return e
	}
	v.committed = a.CommittedThroughSeq
	v.expectedSeq, v.expectedHash = 0, ""
	return nil
}
