package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

func (s *Service) CreateReport(register, typ, tenant string) (Operation, error) {
	if !contains([]string{"X", "Z", "KLEN", "FISCAL_MEMORY", "OPERATOR", "DEPARTMENT", "PLU"}, typ) {
		return Operation{}, errors.New("unsupported report")
	}
	op, err := s.reserveFiscalOperation(register, typ, tenant)
	if err != nil {
		return op, err
	}
	op = s.executeReservedFiscalOperation(op, register, tenant)
	if op.State != "FISCALIZED" {
		return op, s.repo.CommitOperationEvent(op, fiscalCommandEvent(register, op))
	}
	reportID, err := newUUID()
	if err != nil {
		return op, err
	}
	artifactID, err := newUUID()
	if err != nil {
		return op, err
	}
	now := time.Now().UTC()
	payload := map[string]any{"report_id": reportID, "register_id": register, "type": typ, "operation_id": op.ID, "fiscal_reference": op.FiscalReference, "official_currency": "EUR", "policy_version": "BG-2026-EUR", "generated_at": now}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	digest := hex.EncodeToString(sum[:])
	data := map[string]any{"register_id": register, "type": typ, "state": "COMPLETED", "requested_at": now, "completed_at": now, "policy_version": "BG-2026-EUR", "official_currency": "EUR", "artifacts": []any{map[string]any{"artifact_id": artifactID, "media_type": "application/json", "sha256": digest, "size": len(b), "created_at": now}}}
	r := ResourceRecord{Kind: "report", TenantID: tenant, ID: reportID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}
	completed := WebhookEvent{EventID: "event-report-" + reportID, EventType: "register.report.completed", APIVersion: "2026-08-07", TenantID: tenant, ResourceID: reportID, ResourceVersion: r.Version, OccurredAt: now, Data: map[string]any{"report_id": reportID, "register_id": register, "operation_id": op.ID, "type": typ, "artifact_id": artifactID, "fiscal_reference": op.FiscalReference}}
	events := []OutboxItem{fiscalCommandEvent(register, op), {ID: completed.EventID, Event: completed, NextAttempt: now}}
	return op, s.repo.CommitResourceArtifactsOperationEvents(r, op, map[string][]byte{artifactID: b}, events)
}
func (s *Service) Reports(tenant string) []map[string]any { return s.ListResources("report", tenant) }
func (s *Service) Report(id, tenant string) (map[string]any, error) {
	return s.GetResource("report", id, tenant)
}
func (s *Service) ReportArtifact(reportID, artifactID, tenant string) ([]byte, error) {
	r, err := s.repo.Resource("report", reportID)
	if err != nil || r.TenantID != tenant {
		return nil, ErrNotFound
	}
	found := false
	if list, ok := r.Data["artifacts"].([]any); ok {
		for _, x := range list {
			if m, ok := x.(map[string]any); ok && m["artifact_id"] == artifactID {
				found = true
			}
		}
	}
	if !found {
		return nil, ErrNotFound
	}
	return s.repo.Artifact(artifactID, tenant)
}
func (s *Service) AuditEvents(tenant string) []AuditEvent {
	items := s.repo.AuditEvents(tenant)
	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurredAt.Equal(items[j].OccurredAt) {
			return items[i].EventID < items[j].EventID
		}
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items
}
