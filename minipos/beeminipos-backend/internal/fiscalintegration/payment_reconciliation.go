package fiscalintegration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type PaymentReconciliationDecision struct {
	OperationID       string `json:"operation_id"`
	PaymentID         string `json:"payment_id"`
	ObservedState     string `json:"observed_state"`
	TerminalReference string `json:"terminal_reference,omitempty"`
	RRN               string `json:"rrn,omitempty"`
	AuthCode          string `json:"auth_code,omitempty"`
	ReversalReference string `json:"reversal_reference,omitempty"`
	ReversalRRN       string `json:"reversal_rrn,omitempty"`
	ReversalAuthCode  string `json:"reversal_auth_code,omitempty"`
	EvidenceReference string `json:"evidence_reference"`
	EvidenceSHA256    string `json:"evidence_sha256"`
}
type PaymentReconciliationDelegation struct {
	SourceOperatorID string                        `json:"source_operator_id"`
	SourceRegisterID string                        `json:"source_register_id"`
	IdentityIssuer   string                        `json:"identity_issuer"`
	IdentitySubject  string                        `json:"identity_subject"`
	SourceSessionID  string                        `json:"source_session_id"`
	AppInstanceID    string                        `json:"app_instance_id"`
	SourceExpiresAt  time.Time                     `json:"source_expires_at"`
	Decision         PaymentReconciliationDecision `json:"decision"`
}
type PaymentReconciliationCommand struct {
	Command           string                         `json:"command"`
	ClientOperationID string                         `json:"client_operation_id"`
	Authorization     ExecutionAuthorizationEnvelope `json:"reconciliation_authorization"`
}

func (c *Client) AuthorizePaymentReconciliation(ctx context.Context, company string, in PaymentReconciliationDelegation, key string) (PaymentReconciliationCommand, error) {
	token, tenant, e := c.fiscalReadCredential(ctx, company)
	if e != nil {
		return PaymentReconciliationCommand{}, e
	}
	var out PaymentReconciliationCommand
	if e = c.call(ctx, http.MethodPost, "/integration/v1/payment-reconciliations:authorize", token, key, nil, in, &out); e != nil {
		return out, e
	}
	var grant struct {
		TenantID string `json:"tenant_id"`
		ActorID  string `json:"actor_id"`
		Expected struct {
			OperationID string `json:"operation_id"`
			PaymentID   string `json:"payment_id"`
		} `json:"expected"`
	}
	raw, e := base64.RawURLEncoding.DecodeString(out.Authorization.Payload)
	identity, e2 := c.ExecutionIdentity(ctx, company, in.IdentityIssuer, in.IdentitySubject)
	if e != nil || e2 != nil || json.Unmarshal(raw, &grant) != nil || out.Command != "PAYMENT_RECONCILE" || grant.TenantID != tenant || grant.ActorID != identity.ActorSubject || grant.Expected.OperationID != in.Decision.OperationID || grant.Expected.PaymentID != in.Decision.PaymentID {
		return PaymentReconciliationCommand{}, errors.New("reconciliation authority identity mismatch")
	}
	return out, nil
}
