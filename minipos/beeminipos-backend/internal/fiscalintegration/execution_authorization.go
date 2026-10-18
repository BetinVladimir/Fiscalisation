package fiscalintegration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type ExecutionAuthorizationEnvelope struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Payload   string `json:"payload"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}
type ExecutionAuthorizationRequest struct {
	SourceOperatorID                string          `json:"source_operator_id"`
	SourceRegisterID                string          `json:"source_register_id"`
	IdentityIssuer                  string          `json:"identity_issuer"`
	IdentitySubject                 string          `json:"identity_subject"`
	SourceSessionID                 string          `json:"source_session_id"`
	AppInstanceID                   string          `json:"app_instance_id"`
	SourceExpiresAt                 time.Time       `json:"source_expires_at"`
	ClientSigningPublicKeyDERBase64 string          `json:"client_signing_public_key_der_base64"`
	ServerSaleID                    string          `json:"server_sale_id,omitempty"`
	SalePreparation                 json.RawMessage `json:"sale_preparation,omitempty"`
}
type ExecutionAuthorizationResponse struct {
	Authorization      ExecutionAuthorizationEnvelope `json:"authorization"`
	WorkstationSession json.RawMessage                `json:"workstation_session"`
	CanonicalPayload   map[string]any                 `json:"canonical_payload,omitempty"`
}

func (c *Client) IssueExecutionAuthorization(ctx context.Context, companyID string, in ExecutionAuthorizationRequest, idempotency string) (ExecutionAuthorizationResponse, error) {
	token, tenant, err := c.fiscalReadCredential(ctx, companyID)
	if err != nil {
		return ExecutionAuthorizationResponse{}, err
	}
	var out ExecutionAuthorizationResponse
	if err = c.call(ctx, http.MethodPost, "/integration/v1/execution-authorizations", token, idempotency, nil, in, &out); err != nil {
		return out, err
	}
	var claims struct {
		TenantID      string `json:"tenant_id"`
		AppInstanceID string `json:"app_instance_id"`
		ClientKey     string `json:"client_signing_public_key_der_base64"`
	}
	raw, e := base64.RawURLEncoding.DecodeString(out.Authorization.Payload)
	if e != nil || json.Unmarshal(raw, &claims) != nil || claims.TenantID != tenant || claims.AppInstanceID != in.AppInstanceID || claims.ClientKey != in.ClientSigningPublicKeyDERBase64 {
		return ExecutionAuthorizationResponse{}, errors.New("fiscal authorization identity mismatch")
	}
	return out, nil
}

type ExecutionIdentityContext struct {
	TenantID        string `json:"tenant_id"`
	ActorSubject    string `json:"actor_subject"`
	SourceSessionID string `json:"source_session_id"`
	AppInstanceID   string `json:"app_instance_id"`
}

func (c *Client) ExecutionIdentity(ctx context.Context, company, issuer, subject string) (ExecutionIdentityContext, error) {
	_, tenant, err := c.fiscalReadCredential(ctx, company)
	if err != nil {
		return ExecutionIdentityContext{}, err
	}
	var system string
	if err = c.db.QueryRowContext(ctx, `select external_system_id from minipos.company_fiscal_bindings where company_id=$1 and status='ACTIVE'`, company).Scan(&system); err != nil {
		return ExecutionIdentityContext{}, err
	}
	if issuer == "" || subject == "" || system == "" {
		return ExecutionIdentityContext{}, errors.New("source identity required")
	}
	raw, _ := json.Marshal([]string{system, issuer, subject})
	return ExecutionIdentityContext{TenantID: tenant, ActorSubject: "integration:" + base64.RawURLEncoding.EncodeToString(raw)}, nil
}

func (c *Client) DispatchExecutionAuthorization(ctx context.Context, company, authorizationID string) (FiscalOperation, error) {
	token, tenant, err := c.fiscalReadCredential(ctx, company)
	if err != nil {
		return FiscalOperation{}, err
	}
	var out FiscalOperation
	err = c.call(ctx, http.MethodPost, "/integration/v1/execution-authorizations:dispatch", token, "", nil, map[string]any{"authorization_id": authorizationID}, &out)
	if err != nil {
		return out, err
	}
	if out.TenantID != tenant || out.OperationID == "" || out.ClientOperationID != out.OperationID {
		return FiscalOperation{}, errors.New("dispatch fiscal identity mismatch")
	}
	return out, nil
}
