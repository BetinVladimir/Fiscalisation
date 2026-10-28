package fiscalintegration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var allowedAlgorithms = map[string]struct{}{
	"ES256": {},
	"ES384": {},
	"ES512": {},
}

// ErrDispatchOutcomeUnknown is returned when the network request was sent but
// the response was lost (timeout, connection reset). Invariant 2: UNKNOWN ≠ FAILED.
var ErrDispatchOutcomeUnknown = errors.New("dispatch outcome unknown")

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
	if _, ok := allowedAlgorithms[out.Authorization.Algorithm]; !ok {
		return ExecutionAuthorizationResponse{}, fmt.Errorf("fiscal authorization algorithm %q not allowed", out.Authorization.Algorithm)
	}
	raw, e := base64.RawURLEncoding.DecodeString(out.Authorization.Payload)
	if e != nil {
		return ExecutionAuthorizationResponse{}, errors.New("fiscal authorization payload not base64")
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != out.Authorization.SHA256 {
		return ExecutionAuthorizationResponse{}, errors.New("fiscal authorization sha256 mismatch")
	}
	var claims struct {
		TenantID      string `json:"tenant_id"`
		AppInstanceID string `json:"app_instance_id"`
		ClientKey     string `json:"client_signing_public_key_der_base64"`
		ExpiresAt     string `json:"expires_at"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.TenantID != tenant || claims.AppInstanceID != in.AppInstanceID || claims.ClientKey != in.ClientSigningPublicKeyDERBase64 {
		return ExecutionAuthorizationResponse{}, errors.New("fiscal authorization identity mismatch")
	}
	if exp, e2 := time.Parse(time.RFC3339, claims.ExpiresAt); e2 != nil || exp.Before(time.Now()) {
		return ExecutionAuthorizationResponse{}, errors.New("fiscal authorization expired or missing expires_at")
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
	err = c.call(ctx, http.MethodPost, "/integration/v1/execution-authorizations:dispatch", token, "dispatch-"+authorizationID, nil, map[string]any{"authorization_id": authorizationID}, &out)
	if err != nil {
		// Network or timeout errors leave outcome unknown; caller must not retry
		// with a new key (invariant 2: UNKNOWN ≠ FAILED).
		var upstream *HTTPStatusError
		if !errors.As(err, &upstream) {
			return FiscalOperation{}, fmt.Errorf("%w: %v", ErrDispatchOutcomeUnknown, err)
		}
		return out, err
	}
	if out.TenantID != tenant || out.OperationID == "" || out.ClientOperationID != out.OperationID {
		return FiscalOperation{}, errors.New("dispatch fiscal identity mismatch")
	}
	return out, nil
}
