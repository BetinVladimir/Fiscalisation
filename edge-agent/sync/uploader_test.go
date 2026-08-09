package sync

import (
	"context"
	"encoding/json"
	"errors"
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

func TestUploaderFaultMatrixNeverAcknowledgesAmbiguousFailure(t *testing.T) {
	tests := []struct {
		name     string
		response func(Batch, []byte) (*http.Response, error)
	}{
		{"transport loss after send", func(Batch, []byte) (*http.Response, error) {
			return nil, errors.New("connection reset")
		}},
		{"backend rollback", func(Batch, []byte) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"rollback"}`))}, nil
		}},
		{"partial signed acknowledgement", func(batch Batch, key []byte) (*http.Response, error) {
			ack := sign(Ack{AckID: "partial", EdgeID: batch.EdgeID, CommittedThroughSeq: batch.LastSeq - 1, CommittedEventHash: batch.Events[0].EventHash, CommittedAt: time.Now().UTC(), OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)
			body, _ := json.Marshal(ack)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
		}},
		{"malformed acknowledgement", func(Batch, []byte) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"unknown":true}`))}, nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			j, err := journal.Open(filepath.Join(t.TempDir(), "fault.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer j.Close()
			_, _ = j.Append("operation-1", "COMMAND_DURABLE", map[string]any{"state": "ACCEPTED"})
			_, _ = j.Append("operation-1", "COMMAND_RESULT", map[string]any{"state": "FISCALIZED"})
			key := []byte("01234567890123456789012345678901")
			u, _ := NewUploader(j, "edge-1", "device-1", key, "https://fiscal.example.test/public/v1/edge-sync/batches", "")
			calls := 0
			u.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				var batch Batch
				if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
					t.Fatal(err)
				}
				if calls == 1 {
					return test.response(batch, key)
				}
				ack := sign(Ack{AckID: "recovered", EdgeID: batch.EdgeID, CommittedThroughSeq: batch.LastSeq, CommittedEventHash: batch.Events[len(batch.Events)-1].EventHash, CommittedAt: time.Now().UTC(), OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)
				body, _ := json.Marshal(ack)
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
			})}
			if uploaded, err := u.UploadOnce(context.Background(), 100); err == nil || uploaded {
				t.Fatal("ambiguous/partial failure was accepted", uploaded, err)
			}
			for _, event := range j.Events() {
				if event.AcknowledgedAt != nil {
					t.Fatal("failure acknowledged journal data")
				}
			}
			if uploaded, err := u.UploadOnce(context.Background(), 100); err != nil || !uploaded {
				t.Fatal("same idempotent batch did not recover", uploaded, err)
			}
			if calls != 2 {
				t.Fatal("unexpected upload count", calls)
			}
		})
	}
}

func TestUploaderPersistsExactPendingBatchAcrossRestartAndNewEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending-restart.sqlite")
	key := []byte("01234567890123456789012345678901")
	j, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := j.Append("operation-1", "COMMAND_RESULT", map[string]any{"state": "FISCALIZED"})
	u, _ := NewUploader(j, "edge-1", "device-1", key, "https://fiscal.example.test/public/v1/edge-sync/batches", "")
	var committed Ack
	u.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var batch Batch
		_ = json.NewDecoder(r.Body).Decode(&batch)
		committed = sign(Ack{AckID: "committed-before-loss", EdgeID: batch.EdgeID, CommittedThroughSeq: batch.LastSeq, CommittedEventHash: batch.Events[len(batch.Events)-1].EventHash, CommittedAt: time.Now().UTC(), OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)
		return nil, errors.New("response lost after commit")
	})}
	if uploaded, uploadErr := u.UploadOnce(context.Background(), 100); uploadErr == nil || uploaded {
		t.Fatal("ambiguous first upload accepted")
	}
	if pendingFirst, pendingLast, _, ok := j.SyncPending("edge-1"); !ok || pendingFirst != first.Sequence || pendingLast != first.Sequence {
		t.Fatal("exact pending batch was not persisted")
	}
	second, _ := j.Append("operation-2", "COMMAND_RESULT", map[string]any{"state": "FISCALIZED"})
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	u, _ = NewUploader(j, "edge-1", "device-1", key, "https://fiscal.example.test/public/v1/edge-sync/batches", "")
	calls := 0
	u.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var batch Batch
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Fatal(err)
		}
		if calls == 1 {
			if batch.FirstSeq != first.Sequence || batch.LastSeq != first.Sequence || len(batch.Events) != 1 {
				t.Fatal("restart expanded ambiguous pending batch", batch.FirstSeq, batch.LastSeq, len(batch.Events))
			}
			body, _ := json.Marshal(committed)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
		}
		if batch.FirstSeq != second.Sequence || batch.LastSeq != second.Sequence || len(batch.Events) != 1 {
			t.Fatal("new event was lost after pending ACK", batch.FirstSeq, batch.LastSeq, len(batch.Events))
		}
		ack := sign(Ack{AckID: "second", EdgeID: batch.EdgeID, CommittedThroughSeq: batch.LastSeq, CommittedEventHash: batch.Events[0].EventHash, CommittedAt: time.Now().UTC(), OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)
		body, _ := json.Marshal(ack)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	if uploaded, uploadErr := u.UploadOnce(context.Background(), 100); uploadErr != nil || !uploaded {
		t.Fatal("persisted batch ACK replay failed", uploaded, uploadErr)
	}
	if _, _, _, ok := j.SyncPending("edge-1"); ok {
		t.Fatal("ACK did not atomically clear pending marker")
	}
	if uploaded, uploadErr := u.UploadOnce(context.Background(), 100); uploadErr != nil || !uploaded {
		t.Fatal("new event upload failed", uploaded, uploadErr)
	}
	if calls != 2 || len(j.Unacknowledged(100)) != 0 {
		t.Fatal("unexpected restart recovery result", calls, len(j.Unacknowledged(100)))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
