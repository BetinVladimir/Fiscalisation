package sync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"fiscalisation/edge-agent/journal"
)

type DeviceEventEnvelope struct {
	EventID     string         `json:"event_id" cbor:"event_id"`
	OperationID string         `json:"operation_id" cbor:"operation_id"`
	DeviceID    string         `json:"device_id" cbor:"device_id"`
	JournalSeq  int64          `json:"journal_seq" cbor:"journal_seq"`
	EventType   string         `json:"event_type" cbor:"event_type"`
	OccurredAt  string         `json:"occurred_at" cbor:"occurred_at"`
	Payload     map[string]any `json:"payload" cbor:"payload"`
	PrevHash    *string        `json:"prev_hash" cbor:"prev_hash"`
	EventHash   string         `json:"event_hash" cbor:"event_hash"`
	Signature   *string        `json:"signature" cbor:"signature"`
}
type Batch struct {
	EdgeID                   string                `json:"edge_id" cbor:"edge_id"`
	SchemaVersion            string                `json:"schema_version" cbor:"schema_version"`
	FirstSeq                 int64                 `json:"first_seq" cbor:"first_seq"`
	LastSeq                  int64                 `json:"last_seq" cbor:"last_seq"`
	PreviousAcknowledgedHash *string               `json:"previous_acknowledged_hash" cbor:"previous_acknowledged_hash"`
	Events                   []DeviceEventEnvelope `json:"events" cbor:"events"`
	BatchSHA256              string                `json:"batch_sha256" cbor:"batch_sha256"`
	Signature                string                `json:"signature" cbor:"signature"`
}

func BuildBatch(edgeID, deviceID string, events []journal.Event, previous *string, key []byte) (Batch, error) {
	if edgeID == "" || deviceID == "" || len(key) < 16 || len(events) < 1 || len(events) > 100 {
		return Batch{}, errors.New("invalid sync batch input")
	}
	v := Batch{EdgeID: edgeID, SchemaVersion: "2026-08-07", FirstSeq: events[0].Sequence, LastSeq: events[len(events)-1].Sequence, PreviousAcknowledgedHash: previous, Events: make([]DeviceEventEnvelope, 0, len(events))}
	prev := ""
	if previous != nil {
		prev = *previous
	}
	for i, source := range events {
		if source.Sequence != v.FirstSeq+int64(i) {
			return Batch{}, errors.New("non-contiguous journal events")
		}
		var payload map[string]any
		if json.Unmarshal(source.Payload, &payload) != nil || payload == nil {
			return Batch{}, errors.New("invalid journal payload")
		}
		p := prev
		event := DeviceEventEnvelope{EventID: source.EventID, OperationID: source.OperationID, DeviceID: deviceID, JournalSeq: source.Sequence, EventType: normalizedEventType(source.Type, payload), OccurredAt: source.CreatedAt.UTC().Format(time.RFC3339Nano), Payload: payload}
		if p != "" {
			event.PrevHash = &p
		}
		event.EventHash = eventHash(event)
		prev = event.EventHash
		v.Events = append(v.Events, event)
	}
	v.BatchSHA256 = batchHash(v)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(v.BatchSHA256))
	v.Signature = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	return v, nil
}
func BuildNextBatch(j *journal.Journal, edgeID, deviceID string, limit int, key []byte) (Batch, error) {
	events := j.Unacknowledged(limit)
	if len(events) == 0 {
		return Batch{}, errors.New("no unacknowledged events")
	}
	var previous *string
	if through, hash, ok := j.SyncState(edgeID); ok {
		if events[0].Sequence != through+1 {
			return Batch{}, errors.New("local sync cursor gap")
		}
		previous = &hash
	} else if events[0].Sequence != 1 {
		return Batch{}, errors.New("missing initial sync cursor")
	}
	return BuildBatch(edgeID, deviceID, events, previous, key)
}
func normalizedEventType(raw string, payload map[string]any) string {
	if raw == "COMMAND_DURABLE" {
		return "ACCEPTED"
	}
	if raw == "COMMAND_RESULT" {
		if state, _ := payload["State"].(string); state != "" {
			if state == "FISCAL_RESULT_UNKNOWN" {
				return "UNKNOWN"
			}
			return state
		}
		if state, _ := payload["state"].(string); state != "" {
			return state
		}
	}
	return "SNAPSHOT"
}
func eventHash(v DeviceEventEnvelope) string {
	v.EventHash, v.Signature = "", nil
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func batchHash(v Batch) string {
	v.BatchSHA256, v.Signature = "", ""
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
