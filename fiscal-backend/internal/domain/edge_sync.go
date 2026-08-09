package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
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
type EdgeSyncBatch struct {
	EdgeID                   string                `json:"edge_id" cbor:"edge_id"`
	SchemaVersion            string                `json:"schema_version" cbor:"schema_version"`
	FirstSeq                 int64                 `json:"first_seq" cbor:"first_seq"`
	LastSeq                  int64                 `json:"last_seq" cbor:"last_seq"`
	PreviousAcknowledgedHash *string               `json:"previous_acknowledged_hash" cbor:"previous_acknowledged_hash"`
	Events                   []DeviceEventEnvelope `json:"events" cbor:"events"`
	BatchSHA256              string                `json:"batch_sha256" cbor:"batch_sha256"`
	Signature                string                `json:"signature" cbor:"signature"`
}

func DeviceEventHash(v DeviceEventEnvelope) string {
	v.EventHash, v.Signature = "", nil
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func EdgeBatchHash(v EdgeSyncBatch) string {
	v.BatchSHA256, v.Signature = "", ""
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *Service) SyncBatch(v EdgeSyncBatch) (SyncAck, error) {
	return s.SyncBatchForTenant("", v)
}

func (s *Service) SyncBatchForTenant(tenant string, v EdgeSyncBatch) (SyncAck, error) {
	if len(s.bleSigningKey) < 16 || v.EdgeID == "" || v.SchemaVersion != "2026-08-07" || len(v.Events) < 1 || len(v.Events) > 100 || v.FirstSeq < 1 || v.LastSeq < v.FirstSeq || int64(len(v.Events)) != v.LastSeq-v.FirstSeq+1 {
		return SyncAck{}, errors.New("invalid sync batch")
	}
	expectedPrevious := ""
	if previous, ok := s.repo.LastSyncAck(tenant, v.EdgeID); ok {
		if v.FirstSeq != previous.CommittedThroughSeq+1 {
			return SyncAck{}, errors.New("journal gap")
		}
		expectedPrevious = previous.CommittedEventHash
	} else if v.FirstSeq != 1 {
		return SyncAck{}, errors.New("initial journal gap")
	}
	providedPrevious := ""
	if v.PreviousAcknowledgedHash != nil {
		providedPrevious = *v.PreviousAcknowledgedHash
	}
	if providedPrevious != expectedPrevious || EdgeBatchHash(v) != v.BatchSHA256 {
		return SyncAck{}, errors.New("batch hash chain mismatch")
	}
	sig, err := base64.RawURLEncoding.DecodeString(v.Signature)
	if err != nil {
		return SyncAck{}, errors.New("invalid batch signature")
	}
	m := hmac.New(sha256.New, s.bleSigningKey)
	m.Write([]byte(v.BatchSHA256))
	if !hmac.Equal(sig, m.Sum(nil)) {
		return SyncAck{}, errors.New("invalid batch signature")
	}
	previousHash := expectedPrevious
	resultsByOperation := map[string]SyncOperationResult{}
	pendingByOperation := map[string]EdgePendingCommand{}
	pendingUpserts := []EdgePendingCommand{}
	completedPending := []string{}
	sales := []Sale{}
	operations := []Operation{}
	artifacts := map[string][]byte{}
	outbox := []OutboxItem{}
	allowed := map[string]bool{"ACCEPTED": true, "EXECUTING": true, "FISCALIZED": true, "FAILED": true, "UNKNOWN": true, "SNAPSHOT": true, "SYNC_BATCH": true}
	for i, event := range v.Events {
		occurredAt, occurredErr := time.Parse(time.RFC3339Nano, event.OccurredAt)
		if event.JournalSeq != v.FirstSeq+int64(i) || event.EventID == "" || event.OperationID == "" || event.DeviceID == "" || event.Payload == nil || !allowed[event.EventType] || occurredErr != nil || occurredAt.After(time.Now().UTC().Add(2*time.Minute)) || DeviceEventHash(event) != event.EventHash {
			return SyncAck{}, errors.New("invalid sync event")
		}
		gotPrevious := ""
		if event.PrevHash != nil {
			gotPrevious = *event.PrevHash
		}
		if gotPrevious != previousHash {
			return SyncAck{}, errors.New("event hash chain mismatch")
		}
		previousHash = event.EventHash
		state, _ := event.Payload["state"].(string)
		if state == "" {
			state = event.EventType
		}
		resultsByOperation[event.OperationID] = SyncOperationResult{OperationID: event.OperationID, State: state, Version: event.JournalSeq}
		if event.EventType == "ACCEPTED" {
			if pending, ok := pendingCommandFromEvent(event); ok {
				if tenant != "" && pending.TenantID != tenant {
					return SyncAck{}, errors.New("cross-tenant sync event")
				}
				pendingByOperation[event.OperationID] = pending
				pendingUpserts = append(pendingUpserts, pending)
			}
		}
		if event.EventType == "FISCALIZED" || event.EventType == "UNKNOWN" || event.EventType == "FAILED" {
			pending, ok := pendingByOperation[event.OperationID]
			if !ok {
				pending, err = s.repo.EdgePendingCommand(event.OperationID, tenant)
				ok = err == nil
			}
			if ok {
				sale, operation, artifactID, artifactBody, hook, materializeErr := s.materializeEdgeResult(pending, event)
				if materializeErr != nil {
					return SyncAck{}, materializeErr
				}
				operations = append(operations, operation)
				if sale.ID != "" {
					sales = append(sales, sale)
				}
				if artifactID != "" {
					artifacts[artifactID] = artifactBody
				}
				if hook.ID != "" {
					outbox = append(outbox, hook)
				}
				completedPending = append(completedPending, event.OperationID)
			}
		}
	}
	results := make([]SyncOperationResult, 0, len(resultsByOperation))
	for _, result := range resultsByOperation {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].OperationID < results[j].OperationID })
	now := time.Now().UTC()
	ack := SyncAck{AckID: newID("ack"), EdgeID: v.EdgeID, CommittedThroughSeq: v.LastSeq, CommittedEventHash: previousHash, CommittedAt: now, OperationResults: results, Rejected: []map[string]any{}}
	b, _ := json.Marshal(ack)
	m = hmac.New(sha256.New, s.bleSigningKey)
	m.Write(b)
	ack.Signature = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	return ack, s.repo.CommitEdgeSync(tenant, ack, sales, operations, artifacts, outbox, pendingUpserts, completedPending)
}

type offlineSalePayload struct {
	Currency   string           `json:"currency"`
	ExternalID string           `json:"external_id"`
	OperatorID string           `json:"operator_id"`
	Items      []SaleLine       `json:"items"`
	Payments   []PaymentRequest `json:"payments"`
	Metadata   map[string]any   `json:"metadata"`
}

func pendingCommandFromEvent(event DeviceEventEnvelope) (EdgePendingCommand, bool) {
	command, ok := event.Payload["command"].(map[string]any)
	if !ok {
		return EdgePendingCommand{}, false
	}
	payload, ok := command["payload"].(map[string]any)
	if !ok {
		return EdgePendingCommand{}, false
	}
	acceptedAt, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil {
		return EdgePendingCommand{}, false
	}
	v := EdgePendingCommand{OperationID: stringAny(command, "command_id", "CommandID"), TenantID: stringAny(command, "tenant_id", "TenantID"), RegisterID: stringAny(command, "register_id", "RegisterID"), DeviceID: stringAny(command, "device_id", "DeviceID"), CommandType: stringAny(command, "type", "Type"), Payload: payload, OperationSequence: intAny(event.Payload, "operation_sequence", "OperationSequence"), UNPSequence: intAny(event.Payload, "unp_sequence", "UNPSequence"), AcceptedAt: acceptedAt}
	if v.OperationID != event.OperationID || v.TenantID == "" || v.RegisterID == "" || v.DeviceID != event.DeviceID || v.CommandType == "" || v.OperationSequence < 1 || v.UNPSequence < 1 {
		return EdgePendingCommand{}, false
	}
	return v, true
}

func (s *Service) materializeEdgeResult(p EdgePendingCommand, event DeviceEventEnvelope) (Sale, Operation, string, []byte, OutboxItem, error) {
	finishedAt, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil || finishedAt.Before(p.AcceptedAt) {
		return Sale{}, Operation{}, "", nil, OutboxItem{}, errors.New("invalid Edge result time")
	}
	state := event.EventType
	if state == "UNKNOWN" {
		state = "UNKNOWN"
	}
	ref := stringAny(event.Payload, "fiscal_reference", "FiscalReference")
	op := Operation{ID: p.OperationID, TenantID: p.TenantID, Type: p.CommandType, State: state, Version: event.JournalSeq, FiscalReference: ref, Simulated: false, AllowedActions: []string{}, CreatedAt: p.AcceptedAt, UpdatedAt: finishedAt}
	if state == "UNKNOWN" {
		op.ErrorCode = stringAny(event.Payload, "error_code", "ErrorCode")
		op.AllowedActions = []string{"RECONCILE"}
	}
	if p.CommandType != "FISCAL_SALE" {
		return Sale{}, op, "", nil, OutboxItem{}, nil
	}
	b, _ := json.Marshal(p.Payload)
	var payload offlineSalePayload
	if json.Unmarshal(b, &payload) != nil || payload.Currency != "EUR" || payload.ExternalID == "" || len(payload.OperatorID) != 4 || len(payload.Items) == 0 || len(payload.Payments) == 0 {
		return Sale{}, Operation{}, "", nil, OutboxItem{}, errors.New("invalid offline sale payload")
	}
	sale := Sale{ID: "edge-sale-" + p.OperationID, TenantID: p.TenantID, ExternalID: payload.ExternalID, RegisterID: p.RegisterID, OperatorID: payload.OperatorID, UNP: p.RegisterID + "-" + payload.OperatorID + "-" + pad7(p.UNPSequence), State: "UNKNOWN", Version: event.JournalSeq, Lines: payload.Items, Payments: []PaymentRecord{}, FiscalOperationID: p.OperationID, CreatedAt: p.AcceptedAt, UpdatedAt: finishedAt}
	op.SaleID = sale.ID
	for _, line := range sale.Lines {
		if line.LineID == "" || line.Name == "" || !validMoney(line.UnitPrice) || !validQuantity(line.Quantity) || !s.policy.AllowsTaxGroup(line.TaxGroup, finishedAt) {
			return Sale{}, Operation{}, "", nil, OutboxItem{}, errors.New("invalid offline sale line")
		}
	}
	paid := int64(0)
	seenPayments := map[string]bool{}
	for _, payment := range payload.Payments {
		amount, amountErr := parseFixed(payment.Amount.Amount, 2)
		if payment.PaymentID == "" || seenPayments[payment.PaymentID] || !validMoney(payment.Amount) || amountErr != nil || amount <= 0 || !contains([]string{"CASH", "CARD"}, payment.Type) {
			return Sale{}, Operation{}, "", nil, OutboxItem{}, errors.New("invalid offline payment")
		}
		seenPayments[payment.PaymentID] = true
		paid += amount
		sale.Payments = append(sale.Payments, PaymentRecord{PaymentID: payment.PaymentID, Type: payment.Type, Amount: payment.Amount, FiscalReference: ref, CreatedAt: finishedAt})
	}
	if paid != saleTotal(sale) {
		return Sale{}, Operation{}, "", nil, OutboxItem{}, errors.New("offline payment total mismatch")
	}
	artifactID := ""
	var artifact []byte
	if state == "FISCALIZED" && ref != "" {
		sale.State = "COMPLETED"
		artifactID, err = newUUID()
		if err != nil {
			return Sale{}, Operation{}, "", nil, OutboxItem{}, err
		}
		sale.ReceiptArtifactID = artifactID
		artifact, _ = json.Marshal(map[string]any{"sale_id": sale.ID, "operation_id": op.ID, "unp": sale.UNP, "fiscal_reference": ref, "issued_at": finishedAt, "total": Money{Amount: formatFixed(saleTotal(sale)), Currency: "EUR"}, "lines": sale.Lines, "payments": sale.Payments})
	} else if state == "FAILED" {
		sale.State = "CANCELLED"
	}
	eventType := "fiscal.operation.updated"
	if state == "FISCALIZED" {
		eventType = "fiscal.operation.succeeded"
	} else if state == "FAILED" {
		eventType = "fiscal.operation.failed"
	} else if state == "UNKNOWN" {
		eventType = "fiscal.operation.reconciliation_required"
	}
	hookEvent := WebhookEvent{EventID: "event-edge-" + op.ID + "-" + state, EventType: eventType, APIVersion: "2026-08-07", TenantID: p.TenantID, ResourceID: sale.ID, ResourceVersion: op.Version, OccurredAt: finishedAt, Data: map[string]any{"state": op.State, "operation_id": op.ID, "sale_id": sale.ID, "external_id": sale.ExternalID, "fiscal_reference": ref, "error_code": op.ErrorCode}}
	hook := OutboxItem{ID: hookEvent.EventID, Event: hookEvent, NextAttempt: finishedAt}
	return sale, op, artifactID, artifact, hook, nil
}

func stringAny(v map[string]any, keys ...string) string {
	for _, key := range keys {
		if x, ok := v[key].(string); ok {
			return x
		}
	}
	return ""
}
func intAny(v map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch x := v[key].(type) {
		case float64:
			return int64(x)
		case int64:
			return x
		case uint64:
			return int64(x)
		case int:
			return int64(x)
		}
	}
	return 0
}
