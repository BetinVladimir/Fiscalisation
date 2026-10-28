package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrExecutionDelegationDenied = errors.New("execution delegation denied")

// DelegatedExecutionPrincipal is supplied only by the authenticated integration
// credential boundary. Actor audit headers are deliberately not consulted.
type DelegatedExecutionPrincipal struct {
	TenantID, SystemID, CredentialID string
	CanIssue                         bool
}
type DelegatedExecutionRequest struct {
	SourceOperatorID                string                    `json:"source_operator_id"`
	SourceRegisterID                string                    `json:"source_register_id"`
	IdentityIssuer                  string                    `json:"identity_issuer"`
	IdentitySubject                 string                    `json:"identity_subject"`
	SourceSessionID                 string                    `json:"source_session_id"`
	AppInstanceID                   string                    `json:"app_instance_id"`
	SourceExpiresAt                 time.Time                 `json:"source_expires_at"`
	ClientSigningPublicKeyDERBase64 string                    `json:"client_signing_public_key_der_base64"`
	ServerSaleID                    string                    `json:"server_sale_id,omitempty"`
	SalePreparation                 *DelegatedSalePreparation `json:"sale_preparation,omitempty"`
}
type DelegatedExecutionResponse struct {
	Authorization      ExecutionAuthorizationEnvelope `json:"authorization"`
	WorkstationSession WorkstationSession             `json:"workstation_session"`
	CanonicalPayload   map[string]any                 `json:"canonical_payload,omitempty"`
}

// DelegatedActorSubject is an unambiguous namespace including the authenticated
// source system. Administrators explicitly bind this value to the fiscal operator.
func DelegatedActorSubject(system, issuer, subject string) (string, error) {
	for _, v := range []string{system, issuer, subject} {
		if strings.TrimSpace(v) == "" || len(v) > 1024 {
			return "", errors.New("delegated actor identity required")
		}
	}
	raw, _ := json.Marshal([]string{system, issuer, subject})
	return "integration:" + base64.RawURLEncoding.EncodeToString(raw), nil
}
func (s *Service) IssueDelegatedExecutionAuthorization(p DelegatedExecutionPrincipal, in DelegatedExecutionRequest, idempotency string) (DelegatedExecutionResponse, error) {
	var empty DelegatedExecutionResponse
	now := time.Now().UTC()
	if !p.CanIssue || p.TenantID == "" || p.SystemID == "" || p.CredentialID == "" {
		return empty, fmt.Errorf("%w: execution_authorizations:issue capability required", ErrExecutionDelegationDenied)
	}
	actor, err := DelegatedActorSubject(p.SystemID, in.IdentityIssuer, in.IdentitySubject)
	if err != nil {
		return empty, err
	}
	if idempotency == "" || len(idempotency) > 200 || in.SourceOperatorID == "" || in.SourceRegisterID == "" || in.SourceSessionID == "" || len(in.SourceSessionID) > 256 || in.AppInstanceID == "" || len(in.AppInstanceID) > 256 || in.ClientSigningPublicKeyDERBase64 == "" || !now.Before(in.SourceExpiresAt) {
		return empty, errors.New("authenticated source session required")
	}
	find := func(kind, source string) (ResourceRecord, error) {
		var selected ResourceRecord
		for _, v := range s.repo.Resources(kind, p.TenantID) {
			if stringField(v.Data, "external_system_id") == p.SystemID && stringField(v.Data, "external_source_id") == source {
				if selected.ID != "" {
					return ResourceRecord{}, fmt.Errorf("%w: ambiguous source mapping", ErrExecutionDelegationDenied)
				}
				selected = v
			}
		}
		if selected.ID == "" {
			return selected, fmt.Errorf("%w: synchronized source mapping required", ErrExecutionDelegationDenied)
		}
		return selected, nil
	}
	operator, err := find("operator", in.SourceOperatorID)
	if err != nil {
		return empty, err
	}
	register, err := find("register", in.SourceRegisterID)
	if err != nil {
		return empty, err
	}
	if stringField(operator.Data, "actor_subject") != actor || !s.activeOperatorForTenant(stringField(operator.Data, "code"), p.TenantID, now) {
		return empty, fmt.Errorf("%w: administrator namespaced identity mapping required", ErrExecutionDelegationDenied)
	}
	// The source session identity selects one persisted route and actor. A changed
	// body cannot repurpose a session created for another operator or app instance.
	identity := edgeDigest([]string{p.TenantID, p.SystemID, in.SourceSessionID, in.AppInstanceID, register.ID})
	raw := identity[:32]
	sessionID := fmt.Sprintf("%s-%s-4%s-a%s-%s", raw[:8], raw[8:12], raw[13:16], raw[17:20], raw[20:32])
	fingerprint := edgeDigest(map[string]any{"tenant": p.TenantID, "system": p.SystemID, "source_session": in.SourceSessionID, "app": in.AppInstanceID, "actor": actor, "operator": operator.ID, "register": register.ID, "source_exp": in.SourceExpiresAt.UTC().Format(time.RFC3339Nano)})
	sessionResource, lookupErr := s.repo.Resource("workstation_session", sessionID)
	if lookupErr != nil {
		if _, err = s.activeFiscalDeviceSnapshot(register.ID, p.TenantID, now); err != nil {
			return empty, err
		}
		expiry := in.SourceExpiresAt.UTC()
		if limit := now.Add(8 * time.Hour); limit.Before(expiry) {
			expiry = limit
		}
		session := WorkstationSession{SessionID: sessionID, TenantID: p.TenantID, WorkstationID: register.ID, OperatorID: operator.ID, OperatorCode: stringField(operator.Data, "code"), AppInstanceID: in.AppInstanceID, ActorSubject: actor, ExpiresAt: expiry, CreatedAt: now}
		data := asMap(session)
		data["actor_subject"] = actor
		data["delegated_context_sha256"] = fingerprint
		data["delegated_system_id"] = p.SystemID
		sessionResource = ResourceRecord{Kind: "workstation_session", ID: sessionID, TenantID: p.TenantID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}
		if err = s.repo.PutResource(sessionResource); err != nil {
			sessionResource, lookupErr = s.repo.Resource("workstation_session", sessionID)
			if lookupErr != nil {
				return empty, err
			}
		}
	}
	if sessionResource.TenantID != p.TenantID || stringField(sessionResource.Data, "delegated_context_sha256") != fingerprint {
		return empty, errors.New("delegated session context conflict")
	}
	session, err := s.WorkstationSession(sessionID, register.ID, p.TenantID)
	if err != nil {
		return empty, err
	}
	if in.SalePreparation != nil {
		if in.ServerSaleID != "" {
			return empty, errors.New("server sale cannot accompany source preparation")
		}
		sale, e := s.prepareDelegatedSale(p, *in.SalePreparation, session)
		if e != nil {
			return empty, e
		}
		in.ServerSaleID = sale.ID
	}
	// Include the credential in the replay namespace: one source cannot consume or
	// overwrite another credential's request. Mutable actor headers are excluded.
	replayKey := "delegated-execution:" + edgeDigest([]string{p.TenantID, p.SystemID, p.CredentialID, idempotency})
	requestHash := edgeDigest(in)

	if replay, exists := s.repo.Replay(replayKey); exists {
		if replay.Hash != requestHash || replay.Pending || replay.Status != 201 {
			return empty, errors.New("delegated request identity conflict")
		}
		var out DelegatedExecutionResponse
		if json.Unmarshal(replay.Body, &out) != nil {
			return empty, errors.New("delegated replay invalid")
		}
		return out, nil
	}
	out := DelegatedExecutionResponse{WorkstationSession: session}
	if in.ServerSaleID != "" {
		sale, e := s.repo.Sale(in.ServerSaleID)
		if e != nil {
			return empty, e
		}
		out.CanonicalPayload, err = BuildFinalizePayload(sale, operator.ID, session.OperatorCode)
		if err != nil {
			return empty, err
		}
	}
	writer, ok := s.repo.(interface {
		CommitDelegatedAuthorization(ResourceRecord, string, string, []byte) (ReplayRecord, error)
	})
	if !ok {
		return empty, errors.New("atomic delegated issuance unavailable")
	}
	_, err = s.issueExecutionAuthorization(sessionID, in.ClientSigningPublicKeyDERBase64, p.TenantID, actor, func(record ResourceRecord, envelope ExecutionAuthorizationEnvelope) error {
		out.Authorization = envelope
		if out.CanonicalPayload != nil {
			raw, e := base64.RawURLEncoding.DecodeString(envelope.Payload)
			var auth ExecutionAuthorization
			if e != nil || json.Unmarshal(raw, &auth) != nil || auth.UNPAuthority == nil || auth.UNPAuthority.PayloadSHA256 != edgeDigest(out.CanonicalPayload) {
				return errors.New("server plan changed during delegation")
			}
		}
		body, e := json.Marshal(out)
		if e != nil {
			return e
		}
		replay, e := writer.CommitDelegatedAuthorization(record, replayKey, requestHash, body)
		if e != nil {
			return e
		}
		return json.Unmarshal(replay.Body, &out)
	}, in.ServerSaleID)
	if err != nil {
		return empty, err
	}
	return out, nil
}

// Authorization source CAS and the exact response become durable in one store
// generation. A crash cannot leave a pending idempotency claim or mint a second
// stored authorization on retry. Concurrent exact requests return the winner.
func (r *MemoryRepository) CommitDelegatedAuthorization(record ResourceRecord, key, hash string, body []byte) (ReplayRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.persistenceReadyLocked(); err != nil {
		return *new(ReplayRecord), err
	}
	if existing, ok := r.replays[key]; ok {
		if existing.Hash != hash || existing.Pending || existing.Status != 201 {
			return ReplayRecord{}, errors.New("delegated request identity conflict")
		}
		return existing, nil
	}
	replay := ReplayRecord{Hash: hash, Status: 201, Body: append([]byte(nil), body...)}
	r.replays[key] = replay
	if err := r.putResourceLocked(record); err != nil {
		if r.store != nil {
			r.restoreLocked()
		} else {
			delete(r.replays, key)
		}
		return ReplayRecord{}, err
	}
	return replay, nil
}
