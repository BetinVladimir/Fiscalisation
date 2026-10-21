package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrReconciliationDelegationDenied = errors.New("reconciliation delegation denied")

type ReconciliationAdministratorBinding struct {
	OperatorID       string `json:"operator_id"`
	ExternalSystemID string `json:"external_system_id"`
	SourceOperatorID string `json:"source_operator_id"`
	IdentityIssuer   string `json:"identity_issuer"`
	IdentitySubject  string `json:"identity_subject"`
	Enabled          bool   `json:"enabled"`
	ExpectedVersion  int64  `json:"expected_version"`
}

func (s *Service) BindReconciliationAdministrator(in ReconciliationAdministratorBinding, tenant, administrator string) error {
	actor, e := DelegatedActorSubject(in.ExternalSystemID, in.IdentityIssuer, in.IdentitySubject)
	if e != nil || administrator == "" || tenant == "" || in.SourceOperatorID == "" || in.OperatorID == "" || in.ExpectedVersion < 0 {
		return errors.New("explicit administrator mapping required")
	}
	operator, e := s.repo.Resource("operator", in.OperatorID)
	if e != nil || operator.TenantID != tenant || stringAny(operator.Data, "actor_subject") != actor || stringAny(operator.Data, "external_system_id") != in.ExternalSystemID || stringAny(operator.Data, "external_source_id") != in.SourceOperatorID {
		return errors.New("namespaced operator mapping required")
	}
	writer, ok := s.repo.(interface {
		bindReconciliationAdministrator(ReconciliationAdministratorBinding, string, string, string) error
	})
	if !ok {
		return errors.New("atomic administrator mapping unavailable")
	}
	return writer.bindReconciliationAdministrator(in, tenant, administrator, actor)
}
func (r *MemoryRepository) bindReconciliationAdministrator(in ReconciliationAdministratorBinding, tenant, administrator, actor string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	operator, ok := r.resources[resourceKey("operator", in.OperatorID)]
	if !ok || operator.TenantID != tenant || stringAny(operator.Data, "actor_subject") != actor || stringAny(operator.Data, "external_system_id") != in.ExternalSystemID || stringAny(operator.Data, "external_source_id") != in.SourceOperatorID {
		return errors.New("operator mapping changed")
	}
	id := edgeDigest([]string{tenant, actor})
	key := resourceKey("reconciliation_admin_binding", id)
	old, exists := r.resources[key]
	if (!exists && in.ExpectedVersion != 0) || (exists && (old.TenantID != tenant || old.Version != in.ExpectedVersion)) {
		return errors.New("administrator mapping version conflict")
	}
	now := time.Now().UTC()
	next := ResourceRecord{Kind: "reconciliation_admin_binding", ID: id, TenantID: tenant, Version: in.ExpectedVersion + 1, Data: asMap(in), CreatedAt: now, UpdatedAt: now}
	if exists {
		next.CreatedAt = old.CreatedAt
	}
	next.Data["actor_subject"] = actor
	priorAudit := len(r.audit)
	r.resources[key] = next
	r.appendAuditLocked(tenant, administrator, "RECONCILIATION_ADMIN_BOUND", "reconciliation_admin_binding", id, "", old.Data, next.Data)
	if e := r.persistLocked(); e != nil {
		if r.store != nil {
			r.restoreLocked()
		} else {
			if exists {
				r.resources[key] = old
			} else {
				delete(r.resources, key)
			}
			r.audit = r.audit[:priorAudit]
		}
		return e
	}
	return nil
}

type DelegatedReconciliationPrincipal struct {
	TenantID, SystemID, CredentialID string
	CanAuthorize                     bool
}
type DelegatedReconciliationRequest struct {
	SourceOperatorID string                       `json:"source_operator_id"`
	SourceRegisterID string                       `json:"source_register_id"`
	IdentityIssuer   string                       `json:"identity_issuer"`
	IdentitySubject  string                       `json:"identity_subject"`
	SourceSessionID  string                       `json:"source_session_id"`
	AppInstanceID    string                       `json:"app_instance_id"`
	SourceExpiresAt  time.Time                    `json:"source_expires_at"`
	Decision         PaymentReconciliationRequest `json:"decision"`
}

func (s *Service) IssueDelegatedPaymentReconciliation(p DelegatedReconciliationPrincipal, in DelegatedReconciliationRequest, idempotency string) (PaymentReconciliationCommand, error) {
	var empty PaymentReconciliationCommand
	stage := "principal"
	deny := func() (PaymentReconciliationCommand, error) {
		return empty, fmt.Errorf("%w: %s", ErrReconciliationDelegationDenied, stage)
	}
	now := time.Now().UTC()
	if !p.CanAuthorize || p.TenantID == "" || p.SystemID == "" || p.CredentialID == "" || idempotency == "" || len(idempotency) > 200 || in.SourceSessionID == "" || in.AppInstanceID == "" || !now.Before(in.SourceExpiresAt) {
		return deny()
	}
	actor, e := DelegatedActorSubject(p.SystemID, in.IdentityIssuer, in.IdentitySubject)
	if e != nil {
		return deny()
	}
	stage = "mapping"
	mapping, e := s.repo.Resource("reconciliation_admin_binding", edgeDigest([]string{p.TenantID, actor}))
	if e != nil || mapping.TenantID != p.TenantID || mapping.Data["enabled"] != true || stringAny(mapping.Data, "source_operator_id") != in.SourceOperatorID || stringAny(mapping.Data, "external_system_id") != p.SystemID || stringAny(mapping.Data, "actor_subject") != actor {
		return deny()
	}
	stage = "operator"
	operator, e := s.repo.Resource("operator", stringAny(mapping.Data, "operator_id"))
	if e != nil || operator.TenantID != p.TenantID || stringAny(operator.Data, "actor_subject") != actor || stringAny(operator.Data, "external_source_id") != in.SourceOperatorID || stringAny(operator.Data, "external_system_id") != p.SystemID || !s.activeOperatorForTenant(stringAny(operator.Data, "code"), p.TenantID, now) {
		return deny()
	}
	stage = "register"
	var register ResourceRecord
	for _, r := range s.repo.Resources("register", p.TenantID) {
		if stringAny(r.Data, "external_system_id") == p.SystemID && stringAny(r.Data, "external_source_id") == in.SourceRegisterID {
			if register.ID != "" {
				return deny()
			}
			register = r
		}
	}
	if register.ID == "" {
		return deny()
	}
	identity := edgeDigest([]string{p.TenantID, p.SystemID, in.SourceSessionID, in.AppInstanceID, register.ID})
	raw := identity[:32]
	sessionID := fmt.Sprintf("%s-%s-4%s-a%s-%s", raw[:8], raw[8:12], raw[13:16], raw[17:20], raw[20:32])
	stage = "session"
	session, e := s.WorkstationSession(sessionID, register.ID, p.TenantID)
	if e != nil || session.OperatorID != operator.ID || session.AppInstanceID != in.AppInstanceID {
		return deny()
	}
	stage = "session-source"
	record, e := s.repo.Resource("workstation_session", sessionID)
	if e != nil || stringAny(record.Data, "delegated_system_id") != p.SystemID || stringAny(record.Data, "actor_subject") != actor {
		return deny()
	}
	recordVersion := record.Version
	stage = "payment-context"
	pending, _, _, _, e := s.paymentReconciliationContext(in.Decision.OperationID, in.Decision.PaymentID, p.TenantID)
	if e != nil || pending.RegisterID != register.ID {
		return deny()
	}
	key := "delegated-reconciliation:" + edgeDigest([]string{p.TenantID, p.SystemID, p.CredentialID, idempotency})
	hash := edgeDigest(in)
	if replay, ok := s.repo.Replay(key); ok {
		if replay.Hash != hash || replay.Pending || replay.Status != 201 {
			return empty, errors.New("reconciliation request identity conflict")
		}
		var out PaymentReconciliationCommand
		if json.Unmarshal(replay.Body, &out) != nil {
			return empty, errors.New("invalid reconciliation replay")
		}
		return out, nil
	}
	writer, ok := s.repo.(interface {
		CommitDelegatedAuthorization(ResourceRecord, string, string, []byte) (ReplayRecord, error)
	})
	if !ok {
		return empty, errors.New("atomic delegated reconciliation unavailable")
	}
	expiry := in.SourceExpiresAt
	if session.ExpiresAt.Before(expiry) {
		expiry = session.ExpiresAt
	}
	if activeTo := stringAny(operator.Data, "active_to"); activeTo != "" {
		end, err := time.Parse(time.RFC3339Nano, activeTo)
		if err != nil {
			return deny()
		}
		if end.Before(expiry) {
			expiry = end
		}
	}
	var result PaymentReconciliationCommand
	_, e = s.issuePaymentReconciliation(in.Decision, p.TenantID, actor, true, expiry, func(record ResourceRecord, command PaymentReconciliationCommand) error {
		record.Data["delegated_system_id"] = p.SystemID
		record.Data["delegated_credential_id"] = p.CredentialID
		record.Data["delegated_session_id"] = sessionID
		record.Data["delegated_admin_binding_id"] = mapping.ID
		record.Data["delegated_admin_binding_version"] = mapping.Version
		record.Data["delegated_issuance_sources"] = map[string]any{resourceKey("reconciliation_admin_binding", mapping.ID): mapping.Version, resourceKey("operator", operator.ID): operator.Version, resourceKey("register", register.ID): register.Version, resourceKey("workstation_session", sessionID): recordVersion}
		body, e := json.Marshal(command)
		if e != nil {
			return e
		}
		replay, e := writer.CommitDelegatedAuthorization(record, key, hash, body)
		if e != nil {
			return e
		}
		return json.Unmarshal(replay.Body, &result)
	})
	return result, e
}
