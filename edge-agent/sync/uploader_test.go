package sync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fiscalisation/edge-agent/journal"
)

func TestUploaderSendsContractBatchAndAppliesBoundAck(t *testing.T) {
	j, err := journal.Open(filepath.Join(t.TempDir(), "upload.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	_, _ = j.Append("operation-1", "COMMAND_RESULT", map[string]any{"state": "FISCALIZED"})
	key := []byte("01234567890123456789012345678901")
	u, err := NewUploader(j, "edge-1", "device-1", key, "https://fiscal.example.test/public/v1/edge-sync/batches", "token")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	u.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("X-Api-Version") != "2026-08-07" || !strings.HasPrefix(r.Header.Get("Idempotency-Key"), "edge-sync-") || r.Header.Get("Authorization") != "Bearer token" {
			t.Error("required sync headers missing")
		}
		var batch Batch
		if json.NewDecoder(r.Body).Decode(&batch) != nil || batchHash(batch) != batch.BatchSHA256 {
			t.Error("invalid uploaded batch")
		}
		ack := sign(Ack{AckID: "ack-1", EdgeID: batch.EdgeID, CommittedThroughSeq: batch.LastSeq, CommittedEventHash: batch.Events[len(batch.Events)-1].EventHash, CommittedAt: time.Now().UTC(), OperationResults: []OperationResult{{OperationID: "operation-1", State: "FISCALIZED", Version: 1}}, Rejected: []map[string]any{}}, key)
		body, _ := json.Marshal(ack)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	uploaded, err := u.UploadOnce(context.Background(), 100)
	if err != nil || !uploaded || calls != 1 || j.Events()[0].AcknowledgedAt == nil {
		t.Fatalf("upload=%v calls=%d err=%v", uploaded, calls, err)
	}
	uploaded, err = u.UploadOnce(context.Background(), 100)
	if err != nil || uploaded || calls != 1 {
		t.Fatalf("empty retry upload=%v calls=%d err=%v", uploaded, calls, err)
	}
}

func TestUploaderRejectsAckForDifferentBatch(t *testing.T) {
	j, _ := journal.Open(filepath.Join(t.TempDir(), "bad-ack.sqlite"))
	defer j.Close()
	_, _ = j.Append("operation-1", "COMMAND_RESULT", map[string]any{"state": "FISCALIZED"})
	key := []byte("01234567890123456789012345678901")
	u, _ := NewUploader(j, "edge-1", "device-1", key, "https://fiscal.example.test/public/v1/edge-sync/batches", "")
	u.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		ack := sign(Ack{AckID: "wrong", EdgeID: "edge-1", CommittedThroughSeq: 99, CommittedEventHash: "wrong", CommittedAt: time.Now().UTC(), OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)
		body, _ := json.Marshal(ack)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	if _, err := u.UploadOnce(context.Background(), 100); err == nil || j.Events()[0].AcknowledgedAt != nil {
		t.Fatal("unbound acknowledgement accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
