package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

type EdgeSyncExpectation struct {
	Cursor            int64
	Hash              string
	SaleVersions      map[string]int64
	OperationVersions map[string]int64
	PendingDigests    map[string]string
}

// All checks run while the same repository lock guards the subsequent commit.
// Persistent repositories additionally use the store generation CAS transaction.
func (r *MemoryRepository) validateEdgeSyncLocked(tenant string, ack SyncAck, sales []Sale, operations []Operation, pending []EdgePendingCommand, expected EdgeSyncExpectation) error {
	key := ack.EdgeID
	if tenant != "" {
		key = tenant + "\n" + ack.EdgeID
	}
	old, exists := r.syncAcks[key]
	if (!exists && (expected.Cursor != 0 || expected.Hash != "")) || (exists && (old.CommittedThroughSeq != expected.Cursor || old.CommittedEventHash != expected.Hash)) || ack.CommittedThroughSeq <= expected.Cursor {
		return errors.New("concurrent Edge sync cursor conflict")
	}
	for id, version := range expected.SaleVersions {
		old, ok := r.sales[id]
		if (!ok && version != 0) || (ok && (version == 0 || old.Version != version || (tenant != "" && old.TenantID != tenant))) {
			return errors.New("concurrent Edge sale version conflict")
		}
	}
	for id, version := range expected.OperationVersions {
		old, ok := r.operations[id]
		if (!ok && version != 0) || (ok && (version == 0 || old.Version != version || (tenant != "" && old.TenantID != tenant))) {
			return errors.New("concurrent Edge operation version conflict")
		}
	}
	for id, digest := range expected.PendingDigests {
		old, ok := r.edgePending[id]
		if (!ok && digest != "") || (ok && (digest == "" || edgeDigest(old) != digest || (tenant != "" && old.TenantID != tenant))) {
			return errors.New("concurrent Edge acceptance/result conflict")
		}
	}
	// Include every staged sale/pending command, not only the current persisted map.
	stagedSales := map[string]Sale{}
	for id, sale := range r.sales {
		stagedSales[id] = sale
	}
	for _, sale := range sales {
		if sale.ID == "" || sale.TenantID == "" || (tenant != "" && sale.TenantID != tenant) {
			return errors.New("Edge sale tenant ownership required")
		}
		for id, other := range stagedSales {
			if id == sale.ID {
				continue
			}
			if other.TenantID == sale.TenantID && ((sale.UNP != "" && other.UNP == sale.UNP) || (sale.ExternalID != "" && other.ExternalID == sale.ExternalID)) {
				return errors.New("Edge sale immutable identifier conflict")
			}
			for _, payment := range sale.Payments {
				for _, existing := range other.Payments {
					if payment.PaymentID == existing.PaymentID {
						return errors.New("Edge payment belongs to another sale")
					}
				}
			}
		}
		stagedSales[sale.ID] = sale
	}
	stagedPending := map[string]EdgePendingCommand{}
	for id, p := range r.edgePending {
		stagedPending[id] = p
	}
	for _, p := range pending {
		if p.Acceptance == nil {
			stagedPending[p.OperationID] = p
			continue
		}
		if p.TenantID != tenant || p.DeviceID != ack.EdgeID {
			return errors.New("v2 pending sync ownership mismatch")
		}
		if old, ok := stagedPending[p.OperationID]; ok && edgeAcceptanceDigest(old) != edgeAcceptanceDigest(p) {
			return errors.New("immutable accepted plan conflict")
		}
		for id, other := range stagedPending {
			if id == p.OperationID || other.Acceptance == nil {
				continue
			}
			if (p.Acceptance.ReceiptSessionID != "" && p.Acceptance.ReceiptSessionID == other.Acceptance.ReceiptSessionID) || (p.CommandType == "SALE_FINALIZE" && other.CommandType == "SALE_FINALIZE" && p.Acceptance.UNP != "" && p.Acceptance.UNP == other.Acceptance.UNP) {
				return errors.New("Edge receipt/UNP reused by another operation")
			}
			payments, _ := p.Payload["payments"].([]any)
			otherPayments, _ := other.Payload["payments"].([]any)
			for _, a := range payments {
				am, _ := a.(map[string]any)
				for _, b := range otherPayments {
					bm, _ := b.(map[string]any)
					if stringAny(am, "payment_id") != "" && stringAny(am, "payment_id") == stringAny(bm, "payment_id") {
						return errors.New("Edge tender reused by another operation")
					}
				}
			}
		}
		if old, ok := stagedPending[p.OperationID]; ok && old.ResultState != "" && old.ResultState != "UNKNOWN" && old.ResultState != "RECOVERY_REQUIRED" && p.ResultDigest != "" && old.ResultDigest != p.ResultDigest {
			return errors.New("terminal Edge result conflict")
		}
		stagedPending[p.OperationID] = p
	}
	for _, op := range operations {
		if tenant != "" && op.TenantID != tenant {
			return errors.New("Edge operation tenant mismatch")
		}
	}
	return nil
}

func immutableResourceKind(kind string) bool {
	return kind == "unp_serial_identity" || kind == "execution_authorization" || kind == "issued_binding" || kind == "execution_authorization_revocation"
}
func immutableResourceEqual(a, b ResourceRecord) bool {
	return a.Kind == b.Kind && a.ID == b.ID && a.TenantID == b.TenantID && edgeDigest(a.Data) == edgeDigest(b.Data)
}

func (r *MemoryRepository) BindOperatorIdentity(id, subject, tenant, actor string, expected int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := resourceKey("operator", id)
	old, ok := r.resources[key]
	if !ok || old.TenantID != tenant || old.Version != expected {
		return errors.New("operator identity version/tenant conflict")
	}
	for _, other := range r.resources {
		if other.Kind == "operator" && other.TenantID == tenant && other.ID != id && stringField(other.Data, "actor_subject") == subject {
			return errors.New("authenticated subject belongs to another operator")
		}
	}
	next := old
	next.Data = cloneMap(old.Data)
	next.Data["actor_subject"] = subject
	next.Version++
	next.UpdatedAt = time.Now().UTC()
	r.resources[key] = next
	r.appendAuditLocked(tenant, actor, "OPERATOR_IDENTITY_BOUND", "operator", id, "", old.Data, next.Data)
	return r.persistLocked()
}

func edgeDeepMap(v map[string]any) map[string]any {
	raw, e := json.Marshal(v)
	if e != nil {
		return nil
	}
	var out map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&out) != nil {
		return nil
	}
	return out
}
func edgeDeepPending(v EdgePendingCommand) EdgePendingCommand {
	raw, e := json.Marshal(v)
	if e != nil {
		return EdgePendingCommand{}
	}
	var out EdgePendingCommand
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&out) != nil {
		return EdgePendingCommand{}
	}
	return out
}

func copyEdgeMap[K comparable, V any](in map[K]V) map[K]V {
	out := make(map[K]V, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
