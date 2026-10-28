package fiscalintegration

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
)

// FiscalOperation is a fiscal command result, distinct from integration-resource
// synchronization Operation. No command submission is performed by this reader.
type FiscalOperation struct {
	OperationID       string    `json:"operation_id"`
	ClientOperationID string    `json:"client_operation_id"`
	ReceiptSessionID  string    `json:"receipt_session_id"`
	TenantID          string    `json:"tenant_id"`
	SaleID            string    `json:"sale_id"`
	RegisterID        string    `json:"register_id"`
	Type              string    `json:"type"`
	State             string    `json:"state"`
	Version           int64     `json:"version"`
	FiscalReference   string    `json:"fiscal_reference,omitempty"`
	ErrorCode         string    `json:"error_code,omitempty"`
	Simulated         bool      `json:"simulated"`
	AllowedActions    []string  `json:"allowed_actions"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (c *Client) GetFiscalOperation(ctx context.Context, companyID, operationID string) (FiscalOperation, error) {
	token, fiscalTenant, err := c.fiscalReadCredential(ctx, companyID)
	if err != nil {
		return FiscalOperation{}, err
	}
	var out FiscalOperation
	err = c.call(ctx, http.MethodGet, "/public/v1/operations/"+url.PathEscape(operationID), token, "", map[string]string{"X-Api-Version": "2026-08-07"}, nil, &out)
	if err != nil {
		return FiscalOperation{}, err
	}
	if out.TenantID != fiscalTenant || out.OperationID != operationID || out.ClientOperationID != operationID {
		return FiscalOperation{}, errors.New("Fiscal operation identity mismatch")
	}
	return out, nil
}

func (c *Client) fiscalReadCredential(ctx context.Context, companyID string) (string, string, error) {
	var encrypted []byte
	var tenant, storedKeyID string
	err := c.db.QueryRowContext(ctx, `select c.ciphertext,b.fiscal_tenant_id,c.encryption_key_id from minipos.company_fiscal_credentials c join minipos.company_fiscal_bindings b on b.id=c.binding_id where b.company_id=$1 and b.status='ACTIVE' and c.status='ACTIVE'`, companyID).Scan(&encrypted, &tenant, &storedKeyID)
	if err != nil {
		return "", "", err
	}
	token, err := c.decryptWithKeyID(storedKeyID, encrypted)
	if err != nil || token == "" || tenant == "" {
		return "", "", errors.New("Fiscal read credential unavailable")
	}
	// Re-wrap with the active key if the credential was encrypted with an older key.
	if c.activeKeyID != "" && storedKeyID != c.activeKeyID {
		if newCT, e := c.encrypt(token); e == nil {
			// $1=storedKeyID $2=activeKeyID $3=encrypted(old) $4=newCT — order matters for test spy capture
			_, _ = c.db.ExecContext(ctx, `update minipos.company_fiscal_credentials set ciphertext=$4,encryption_key_id=$2,updated_at=now() where status='ACTIVE' and encryption_key_id=$1 and ciphertext=$3`, storedKeyID, c.activeKeyID, encrypted, newCT)
		}
	}
	return token, tenant, nil
}

func (c *Client) decryptWithKeyID(keyID string, data []byte) (string, error) {
	if c.keyring != nil {
		aead, ok := c.keyring[keyID]
		if !ok {
			return "", errors.New("unknown encryption key ID: " + keyID)
		}
		n := aead.NonceSize()
		if len(data) < n {
			return "", errors.New("invalid ciphertext")
		}
		p, e := aead.Open(nil, data[:n], data[n:], nil)
		return string(p), e
	}
	// Fallback: single-key mode (keyring not configured).
	return c.decrypt(data)
}

type FiscalSale struct {
	SaleID     string `json:"sale_id"`
	TenantID   string `json:"tenant_id"`
	ExternalID string `json:"external_id"`
	RegisterID string `json:"register_id"`
	OperatorID string `json:"operator_id"`
}

func (c *Client) GetFiscalSale(ctx context.Context, companyID, saleID string) (FiscalSale, error) {
	token, tenant, err := c.fiscalReadCredential(ctx, companyID)
	if err != nil {
		return FiscalSale{}, err
	}
	var out FiscalSale
	err = c.call(ctx, http.MethodGet, "/public/v1/sales/"+url.PathEscape(saleID), token, "", map[string]string{"X-Api-Version": "2026-08-07"}, nil, &out)
	if err != nil {
		return FiscalSale{}, err
	}
	if out.TenantID != tenant || out.SaleID != saleID {
		return FiscalSale{}, errors.New("Fiscal sale identity mismatch")
	}
	return out, nil
}
