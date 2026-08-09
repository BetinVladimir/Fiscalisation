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

// TestAccelerated72HourNetworkFlapSoak models one five-minute sync slot for
// each of 72 hours. It includes offline periods, ambiguous loss after backend
// commit, idempotent ACK replay and a process/SQLite reopen every 24 hours.
func TestAccelerated72HourNetworkFlapSoak(t *testing.T) {
	const slots = 72 * 12
	path := filepath.Join(t.TempDir(), "network-flap-soak.sqlite")
	key := []byte("01234567890123456789012345678901")
	committed := make(map[string]struct{}, slots)
	acks := make(map[string]Ack)
	requests := make(map[string]int)
	duplicateCommits := 0
	lastCommittedHash := ""
	var lastCommittedSequence int64
	currentSlot := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var batch Batch
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			return nil, err
		}
		idempotencyKey := r.Header.Get("Idempotency-Key")
		if idempotencyKey == "" || batchHash(batch) != batch.BatchSHA256 {
			return nil, errors.New("invalid soak batch")
		}
		requests[idempotencyKey]++
		if ack, ok := acks[idempotencyKey]; ok {
			body, _ := json.Marshal(ack)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
		}
		// Every seventh slot is a clean outage: the backend sees nothing.
		if currentSlot%7 == 1 {
			return nil, errors.New("network unavailable")
		}
		for _, event := range batch.Events {
			if _, exists := committed[event.EventID]; exists {
				duplicateCommits++
			}
			committed[event.EventID] = struct{}{}
		}
		ack := sign(Ack{AckID: "soak-" + batch.BatchSHA256[:16], EdgeID: batch.EdgeID, CommittedThroughSeq: batch.LastSeq, CommittedEventHash: batch.Events[len(batch.Events)-1].EventHash, CommittedAt: time.Now().UTC(), OperationResults: []OperationResult{}, Rejected: []map[string]any{}}, key)
		acks[idempotencyKey] = ack
		if batch.LastSeq > lastCommittedSequence {
			lastCommittedSequence = batch.LastSeq
			lastCommittedHash = ack.CommittedEventHash
		}
		// Every eleventh slot loses the response after a durable backend commit.
		if currentSlot%11 == 3 && requests[idempotencyKey] == 1 {
			return nil, errors.New("connection reset after commit")
		}
		body, _ := json.Marshal(ack)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})

	var j *journal.Journal
	var uploader *Uploader
	open := func() {
		var err error
		j, err = journal.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		uploader, err = NewUploader(j, "edge-soak", "device-soak", key, "https://fiscal.example.test/public/v1/edge-sync/batches", "token")
		if err != nil {
			t.Fatal(err)
		}
		uploader.client = &http.Client{Transport: transport}
	}
	open()
	defer func() { _ = j.Close() }()

	for currentSlot = 0; currentSlot < slots; currentSlot++ {
		if _, err := j.Append("soak-operation-"+time.Unix(int64(currentSlot), 0).UTC().Format("150405"), "COMMAND_RESULT", map[string]any{"slot": currentSlot, "state": "FISCALIZED"}); err != nil {
			t.Fatal(err)
		}
		_, _ = uploader.UploadOnce(context.Background(), 17) // failures are expected and retried unchanged
		if currentSlot > 0 && currentSlot%(24*12) == 0 {
			if err := j.Close(); err != nil {
				t.Fatal(err)
			}
			open()
			if !j.Verify() {
				t.Fatal("journal chain failed after daily restart")
			}
		}
	}
	currentSlot = slots
	for attempts := 0; attempts < slots && len(j.Unacknowledged(100)) > 0; attempts++ {
		if _, err := uploader.UploadOnce(context.Background(), 100); err != nil {
			t.Fatal("final drain failed", err)
		}
	}
	events := j.Events()
	if len(events) != slots || len(committed) != slots || duplicateCommits != 0 || len(j.Unacknowledged(100)) != 0 || !j.Verify() {
		t.Fatalf("loss/duplicate detected: events=%d committed=%d duplicates=%d pending=%d", len(events), len(committed), duplicateCommits, len(j.Unacknowledged(100)))
	}
	through, hash, ok := j.SyncState("edge-soak")
	if !ok || through != slots || lastCommittedSequence != slots || hash != lastCommittedHash {
		t.Fatalf("invalid durable sync cursor: ok=%v through=%d", ok, through)
	}
	for key, count := range requests {
		if count > 2 {
			t.Fatalf("unbounded retry for %s: %d", key, count)
		}
	}
}
