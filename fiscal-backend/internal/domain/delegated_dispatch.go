package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

func (s *Service) DispatchDelegatedExecution(p DelegatedExecutionPrincipal, authorizationID string) (Operation, error) {
	if !p.CanIssue || p.TenantID == "" || p.SystemID == "" || p.CredentialID == "" {
		return Operation{}, errors.New("execution_authorizations:issue capability required")
	}
	record, err := s.repo.Resource("execution_authorization", authorizationID)
	if err != nil || record.TenantID != p.TenantID {
		return Operation{}, errors.New("authorization unavailable")
	}
	auth, _, err := executionAuthorizationFromResource(record)
	now := time.Now().Unix()
	if err != nil || now < auth.NBF || now >= auth.Exp || auth.UNPAuthority == nil {
		return Operation{}, errors.New("current server-assigned authorization required")
	}
	sessionResource, err := s.repo.Resource("workstation_session", auth.WorkstationSessionID)
	if err != nil || sessionResource.TenantID != p.TenantID || stringField(sessionResource.Data, "delegated_system_id") != p.SystemID || stringField(sessionResource.Data, "actor_subject") != auth.ActorSubject {
		return Operation{}, errors.New("delegated source session mismatch")
	}
	if _, err = s.WorkstationSession(auth.WorkstationSessionID, auth.RegisterID, p.TenantID); err != nil {
		return Operation{}, err
	}
	operator, err := s.repo.Resource("operator", auth.OperatorID)
	if err != nil || operator.TenantID != p.TenantID || stringField(operator.Data, "actor_subject") != auth.ActorSubject {
		return Operation{}, errors.New("operator identity changed")
	}
	if _, err = s.repo.Resource("execution_authorization_revocation", authorizationID); err == nil {
		return Operation{}, errors.New("authorization revoked")
	}
	sale, err := s.repo.Sale(auth.UNPAuthority.ServerSaleID)
	if err != nil || sale.TenantID != p.TenantID || sale.RegisterID != auth.RegisterID || sale.FiscalOperationID != auth.UNPAuthority.OperationID {
		return Operation{}, errors.New("prepared sale mismatch")
	}
	op, err := s.repo.Operation(auth.UNPAuthority.OperationID)
	if err != nil || op.TenantID != p.TenantID || op.SaleID != sale.ID || op.ReceiptSessionID != auth.UNPAuthority.ReceiptSessionID {
		return Operation{}, errors.New("prepared operation mismatch")
	}
	// Terminal/accepted states are observations, never permission to publish anew.
	if op.State != "EXECUTING" || sale.State != "PAYMENT_PENDING" {
		return op, nil
	}
	payload, err := BuildFinalizePayload(sale, auth.OperatorID, auth.OperatorCode)
	if err != nil || edgeDigest(payload) != auth.UNPAuthority.PayloadSHA256 {
		return Operation{}, errors.New("prepared receipt plan changed")
	}
	driver, err := s.boundDriver(p.TenantID, sale.RegisterID, sale.FiscalDevice)
	if err != nil {
		return Operation{}, err
	}
	queued, ok := driver.(durableQueuedDriver)
	if !ok {
		return Operation{}, errors.New("durable cloud dispatch unavailable")
	}
	command, lookupErr := s.repo.Resource("device_command_outbox", op.ID)
	if lookupErr != nil {
		command, err = queued.Prepare(op, sale, PaymentRequest{})
		if err != nil {
			return Operation{}, err
		}
		if err = validateDelegatedCommand(command, p.TenantID, op.ID, auth.UNPAuthority.PayloadSHA256); err != nil {
			return Operation{}, err
		}
		if err = s.repo.PutResource(command); err != nil {
			command, lookupErr = s.repo.Resource("device_command_outbox", op.ID)
			if lookupErr != nil {
				return Operation{}, err
			}
		}
	}
	if err = validateDelegatedCommand(command, p.TenantID, op.ID, auth.UNPAuthority.PayloadSHA256); err != nil {
		return Operation{}, err
	}
	// Publication loss is represented by the durable command and existing operation.
	// The device deduplicates the same operation/receipt across MQTT and local ingress.
	_ = queued.Publish(command) // best-effort: MQTT publication loss is recovered via durable command on device reconnect
	return op, nil
}

func validateDelegatedCommand(command ResourceRecord, tenant, operation, hash string) error {
	if command.TenantID != tenant || command.ID != operation {
		return errors.New("cloud command ownership mismatch")
	}
	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal([]byte(stringField(command.Data, "body")), &envelope) != nil {
		return errors.New("prepared cloud command invalid")
	}
	var plan any
	decoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	decoder.UseNumber()
	if decoder.Decode(&plan) != nil || edgeDigest(plan) != hash {
		return errors.New("cloud command plan differs from authorization")
	}
	return nil
}
