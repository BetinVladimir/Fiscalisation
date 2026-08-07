package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

func (s *Service) CreateReport(register, typ, tenant string) (Operation, error) {
	if !contains([]string{"X", "Z", "KLEN", "FISCAL_MEMORY", "OPERATOR", "DEPARTMENT", "PLU"}, typ) {
		return Operation{}, errors.New("unsupported report")
	}
	op, err := s.FiscalOperation(register, typ, tenant)
	if err != nil {
		return op, err
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
	if err = s.repo.PutArtifact(artifactID, b); err != nil {
		return op, err
	}
	data := map[string]any{"register_id": register, "type": typ, "state": "COMPLETED", "requested_at": now, "completed_at": now, "policy_version": "BG-2026-EUR", "official_currency": "EUR", "artifacts": []any{map[string]any{"artifact_id": artifactID, "media_type": "application/json", "sha256": digest, "size": len(b), "created_at": now}}}
	r := ResourceRecord{Kind: "report", TenantID: tenant, ID: reportID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}
	if err = s.repo.PutResource(r); err != nil {
		return op, err
	}
	return op, nil
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
	return s.repo.Artifact(artifactID)
}
func (s *Service) AuditEvents(tenant string) []AuditEvent { return s.repo.AuditEvents(tenant) }
