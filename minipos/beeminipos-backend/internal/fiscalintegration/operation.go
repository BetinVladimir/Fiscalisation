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
	var tenant string
	err := c.db.QueryRowContext(ctx, `select c.ciphertext,b.fiscal_tenant_id from minipos.company_fiscal_credentials c join minipos.company_fiscal_bindings b on b.id=c.binding_id where b.company_id=$1 and b.status='ACTIVE' and c.status='ACTIVE'`, companyID).Scan(&encrypted, &tenant)
	if err != nil {
		return "", "", err
	}
	token, err := c.decrypt(encrypted)
	if err != nil || token == "" || tenant == "" {
		return "", "", errors.New("Fiscal read credential unavailable")
	}
	return token, tenant, nil
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
