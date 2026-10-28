package domain

import (
	"fmt"
	"time"
)

func (s *Service) Replay(k string) (ReplayRecord, bool) { return s.repo.Replay(k) }
func (s *Service) ClaimReplay(k, hash string) (ReplayRecord, bool, error) {
	return s.repo.ClaimReplay(k, hash)
}
func (s *Service) PutReplay(k string, v ReplayRecord) error { return s.repo.PutReplay(k, v) }
func (s *Service) PutAmbiguousReplay(k string, v ReplayRecord) error {
	return s.repo.PutAmbiguousReplay(k, v)
}
func (s *Service) QueueFiscalEvent(saleID string, op Operation) error {
	externalID := ""
	if sale, err := s.repo.Sale(saleID); err == nil {
		externalID = sale.ExternalID
	}
	e := WebhookEvent{EventID: "event-" + op.ID, EventType: "fiscal.operation.updated", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: saleID, ResourceVersion: op.Version, OccurredAt: time.Now().UTC(), Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "external_id": externalID, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode}}
	return s.repo.AddOutbox(OutboxItem{ID: e.EventID, Event: e, NextAttempt: time.Now().UTC()})
}
func fiscalOperationEvent(sale Sale, op Operation) OutboxItem {
	at := op.UpdatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	e := WebhookEvent{EventID: "event-" + op.ID, EventType: "fiscal.operation.updated", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: sale.ID, ResourceVersion: op.Version, OccurredAt: at, Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "external_id": sale.ExternalID, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode}}
	return OutboxItem{ID: e.EventID, Event: e, NextAttempt: at}
}
func fiscalCommandEvent(register string, op Operation) OutboxItem {
	at := op.UpdatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	e := WebhookEvent{EventID: "event-" + op.ID, EventType: "fiscal.operation.updated", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: op.ID, ResourceVersion: op.Version, OccurredAt: at, Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "register_id": register, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode}}
	return OutboxItem{ID: e.EventID, Event: e, NextAttempt: at}
}
func reconciliationRequiredEvent(op Operation) OutboxItem {
	at := op.UpdatedAt
	resourceID := op.SaleID
	if resourceID == "" {
		resourceID = op.ID
	}
	e := WebhookEvent{EventID: fmt.Sprintf("event-reconcile-%s-%d", op.ID, op.Version), EventType: "fiscal.operation.reconciliation_required", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: resourceID, ResourceVersion: op.Version, OccurredAt: at, Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "sale_id": op.SaleID, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode, "lookup_only": true}}
	return OutboxItem{ID: e.EventID, Event: e, NextAttempt: at}
}
func (s *Service) PendingOutbox(now time.Time) []OutboxItem { return s.repo.PendingOutbox(now) }
func (s *Service) UpdateOutbox(v OutboxItem) error          { return s.repo.UpdateOutbox(v) }
