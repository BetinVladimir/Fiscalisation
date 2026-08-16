package fiscalintegration

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Client struct {
	db                       *sql.DB
	base, systemToken, keyID string
	aead                     cipher.AEAD
	http                     *http.Client
}
type Enrollment struct {
	Email           string `json:"email"`
	SourceCompanyID string `json:"source_company_id"`
	LegalName       string `json:"legal_name"`
	TaxCountry      string `json:"tax_country"`
	TaxType         string `json:"tax_type"`
	TaxValue        string `json:"tax_value"`
	Address         string `json:"address"`
}
type Challenge struct {
	TemporaryToken string    `json:"temporary_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	ResendAfter    time.Time `json:"resend_after"`
}
type Result struct {
	TenantID        string   `json:"tenant_id"`
	SourceSystemID  string   `json:"source_system_id"`
	SourceCompanyID string   `json:"source_company_id"`
	AccessToken     string   `json:"access_token"`
	Scopes          []string `json:"scopes"`
}
type Operation struct {
	OperationID string    `json:"operation_id"`
	Status      string    `json:"status"`
	StatusURL   string    `json:"status_url"`
	AcceptedAt  time.Time `json:"accepted_at"`
}

func New(databaseURL, publicBase, systemToken, keyBase64, keyID string) (*Client, error) {
	if databaseURL == "" || systemToken == "" {
		return nil, errors.New("Fiscal integration is not configured")
	}
	parsed, e := url.Parse(publicBase)
	if e != nil {
		return nil, e
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	key, e := base64.StdEncoding.DecodeString(keyBase64)
	if e != nil || len(key) != 32 {
		return nil, errors.New("Fiscal credential encryption key must be base64 32 bytes")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	db, e := sql.Open("pgx", databaseURL)
	if e != nil {
		return nil, e
	}
	return &Client{db: db, base: strings.TrimRight(parsed.String(), "/"), systemToken: systemToken, keyID: keyID, aead: aead, http: &http.Client{Timeout: 15 * time.Second}}, nil
}
func (c *Client) Close() error { return c.db.Close() }
func (c *Client) encrypt(v string) ([]byte, error) {
	n := make([]byte, c.aead.NonceSize())
	if _, e := rand.Read(n); e != nil {
		return nil, e
	}
	return c.aead.Seal(n, n, []byte(v), nil), nil
}
func (c *Client) decrypt(v []byte) (string, error) {
	n := c.aead.NonceSize()
	if len(v) < n {
		return "", errors.New("invalid ciphertext")
	}
	p, e := c.aead.Open(nil, v[:n], v[n:], nil)
	return string(p), e
}
func (c *Client) call(ctx context.Context, method, path, token, idempotency string, headers map[string]string, input, out any) error {
	var body io.Reader
	if input != nil {
		b, e := json.Marshal(input)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if idempotency != "" {
		req.Header.Set("Idempotency-Key", idempotency)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if e != nil {
		return e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Fiscal HTTP %d: %s", resp.StatusCode, string(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
func (c *Client) Start(ctx context.Context, companyID, idempotency string, in Enrollment) (Challenge, error) {
	payload := map[string]any{"email": in.Email, "source_company_id": in.SourceCompanyID, "company": map[string]any{"legal_name": in.LegalName, "tax_identifier": map[string]string{"country": in.TaxCountry, "type": in.TaxType, "value": in.TaxValue}, "address": in.Address}}
	var out Challenge
	e := c.call(ctx, http.MethodPost, "/integration/v1/enrollments", c.systemToken, idempotency, nil, payload, &out)
	if e != nil {
		return out, e
	}
	encrypted, e := c.encrypt(out.TemporaryToken)
	if e != nil {
		return out, e
	}
	_, e = c.db.ExecContext(ctx, `insert into minipos_fiscal_enrollment_sessions(company_id,idempotency_key,temporary_token_ciphertext,expires_at,status,tax_country,tax_type,tax_value) values($1,$2,$3,$4,'PENDING',$5,$6,$7) on conflict(company_id,idempotency_key) do update set temporary_token_ciphertext=excluded.temporary_token_ciphertext,expires_at=excluded.expires_at,tax_country=excluded.tax_country,tax_type=excluded.tax_type,tax_value=excluded.tax_value,updated_at=now()`, companyID, idempotency, encrypted, out.ExpiresAt, in.TaxCountry, in.TaxType, in.TaxValue)
	out.TemporaryToken = ""
	return out, e
}
func (c *Client) Verify(ctx context.Context, companyID, idempotency, code string) (Result, error) {
	var encrypted []byte
	e := c.db.QueryRowContext(ctx, `select temporary_token_ciphertext from minipos_fiscal_enrollment_sessions where company_id=$1 and idempotency_key=$2 and status='PENDING' and expires_at>now()`, companyID, idempotency).Scan(&encrypted)
	if e != nil {
		return Result{}, e
	}
	temporary, e := c.decrypt(encrypted)
	if e != nil {
		return Result{}, e
	}
	var out Result
	e = c.call(ctx, http.MethodPost, "/integration/v1/enrollments:verify", temporary, idempotency+"-verify", nil, map[string]string{"code": code}, &out)
	if e != nil {
		return out, e
	}
	return c.saveCredential(ctx, companyID, idempotency, out)
}

func (c *Client) StartRecovery(ctx context.Context, companyID, idempotency, email, country, kind, tax string) (Challenge, error) {
	var out Challenge
	payload := map[string]any{"email": email, "tax_identifier": map[string]string{"country": country, "type": kind, "value": tax}}
	e := c.call(ctx, http.MethodPost, "/integration/v1/credentials:recover", c.systemToken, idempotency, nil, payload, &out)
	if e != nil {
		return out, e
	}
	encrypted, e := c.encrypt(out.TemporaryToken)
	if e != nil {
		return out, e
	}
	_, e = c.db.ExecContext(ctx, `insert into minipos_fiscal_enrollment_sessions(company_id,idempotency_key,temporary_token_ciphertext,expires_at,status,tax_country,tax_type,tax_value) values($1,$2,$3,$4,'PENDING',$5,$6,$7) on conflict(company_id,idempotency_key) do update set temporary_token_ciphertext=excluded.temporary_token_ciphertext,expires_at=excluded.expires_at,status='PENDING',tax_country=excluded.tax_country,tax_type=excluded.tax_type,tax_value=excluded.tax_value,updated_at=now()`, companyID, idempotency, encrypted, out.ExpiresAt, country, kind, tax)
	out.TemporaryToken = ""
	return out, e
}

func (c *Client) VerifyRecovery(ctx context.Context, companyID, idempotency, code string) (Result, error) {
	var encrypted []byte
	e := c.db.QueryRowContext(ctx, `select temporary_token_ciphertext from minipos_fiscal_enrollment_sessions where company_id=$1 and idempotency_key=$2 and status='PENDING' and expires_at>now()`, companyID, idempotency).Scan(&encrypted)
	if e != nil {
		return Result{}, e
	}
	temporary, e := c.decrypt(encrypted)
	if e != nil {
		return Result{}, e
	}
	var out Result
	e = c.call(ctx, http.MethodPost, "/integration/v1/credentials:recover-verify", temporary, idempotency+"-verify", nil, map[string]string{"code": code}, &out)
	if e != nil {
		return out, e
	}
	return c.saveCredential(ctx, companyID, idempotency, out)
}

func (c *Client) saveCredential(ctx context.Context, companyID, idempotency string, out Result) (Result, error) {
	credential, e := c.encrypt(out.AccessToken)
	if e != nil {
		return out, e
	}
	parts := strings.Split(strings.TrimPrefix(out.AccessToken, "tenant_live_"), ".")
	if len(parts) != 2 {
		return out, errors.New("invalid tenant credential")
	}
	fingerprint := sha256.Sum256([]byte(out.AccessToken))
	tx, e := c.db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	var bindingID string
	var before []byte
	_ = tx.QueryRowContext(ctx, `select to_jsonb(b) from company_fiscal_bindings b where company_id=$1 for update`, companyID).Scan(&before)
	e = tx.QueryRowContext(ctx, `insert into company_fiscal_bindings(company_id,external_system_id,source_company_id,fiscal_tenant_id,status) values($1,$2,$3,$4,'ACTIVE') on conflict(company_id) do update set external_system_id=excluded.external_system_id,source_company_id=excluded.source_company_id,fiscal_tenant_id=excluded.fiscal_tenant_id,status='ACTIVE',version=company_fiscal_bindings.version+1,updated_at=now() returning id::text`, companyID, out.SourceSystemID, out.SourceCompanyID, out.TenantID).Scan(&bindingID)
	if e != nil {
		return out, e
	}
	_, e = tx.ExecContext(ctx, `insert into company_fiscal_binding_history(binding_id,actor_type,actor_id,reason,before_redacted,after_redacted) select id,'SERVICE','beeminipos-backend','credential enrollment/recovery',$2,to_jsonb(b) from company_fiscal_bindings b where id=$1`, bindingID, before)
	if e != nil {
		return out, e
	}
	_, e = tx.ExecContext(ctx, `update company_fiscal_credentials set status='REVOKED',revoked_at=now() where binding_id=$1 and status='ACTIVE'`, bindingID)
	if e != nil {
		return out, e
	}
	_, e = tx.ExecContext(ctx, `insert into company_fiscal_credentials(binding_id,credential_id,credential_fingerprint,ciphertext,encryption_key_id,status) values($1,$2,$3,$4,$5,'ACTIVE')`, bindingID, parts[0], hex.EncodeToString(fingerprint[:])[:16], credential, c.keyID)
	if e != nil {
		return out, e
	}
	_, e = tx.ExecContext(ctx, `update minipos_fiscal_enrollment_sessions set status='VERIFIED',updated_at=now() where company_id=$1 and idempotency_key=$2`, companyID, idempotency)
	if e != nil {
		return out, e
	}
	if e = tx.Commit(); e != nil {
		return out, e
	}
	var name, address, taxValue, taxCountry, taxType string
	e = c.db.QueryRowContext(ctx, `select o.name,o.address,coalesce(s.tax_value,o.tax_identifier),coalesce(s.tax_country,'BG'),coalesce(s.tax_type,'EIK') from organizations o left join minipos_fiscal_enrollment_sessions s on s.company_id=o.id and s.idempotency_key=$2 where o.id=$1`, companyID, idempotency).Scan(&name, &address, &taxValue, &taxCountry, &taxType)
	if e == nil {
		_, e = c.PutResource(ctx, companyID, "organization", "", idempotency+"-organization", 1, "SERVICE", "beeminipos-backend", "", map[string]any{"legal_name": name, "address": address, "tax_identifier": map[string]string{"country": taxCountry, "type": taxType, "value": taxValue}, "status": "ACTIVE"})
	}
	out.AccessToken = ""
	if e != nil {
		_, _ = c.db.ExecContext(ctx, `update company_fiscal_bindings set status='DEGRADED',last_error_code='ORGANIZATION_SYNC_FAILED',last_error_detail=$2,updated_at=now() where company_id=$1`, companyID, e.Error())
	}
	// The credential has already been committed. Returning an error here would hide a
	// successful enrollment response and make an idempotent retry unable to recover it.
	return out, nil
}

func (c *Client) ReadyForFiscalOperations(ctx context.Context, companyID, registerID string) bool {
	var ready bool
	e := c.db.QueryRowContext(ctx, `select exists(select 1 from company_fiscal_bindings b join company_fiscal_credentials c on c.binding_id=b.id and c.status='ACTIVE' where b.company_id=$1 and b.status='ACTIVE' and exists(select 1 from company_fiscal_resource_links l where l.binding_id=b.id and l.resource_type='location' and l.sync_status='SUCCEEDED') and exists(select 1 from company_fiscal_resource_links r where r.binding_id=b.id and r.resource_type='register' and r.source_entity_id=$2 and r.sync_status='SUCCEEDED'))`, companyID, registerID).Scan(&ready)
	return e == nil && ready
}
func (c *Client) tenantCredential(ctx context.Context, companyID string) (string, string, error) {
	var encrypted []byte
	var binding string
	e := c.db.QueryRowContext(ctx, `select c.ciphertext,b.id::text from company_fiscal_credentials c join company_fiscal_bindings b on b.id=c.binding_id where b.company_id=$1 and b.status='ACTIVE' and c.status='ACTIVE'`, companyID).Scan(&encrypted, &binding)
	if e != nil {
		return "", "", e
	}
	token, e := c.decrypt(encrypted)
	return token, binding, e
}
func (c *Client) PutResource(ctx context.Context, companyID, resource, sourceID, idempotency string, sourceVersion int64, actorType, actorID, session string, payload any) (Operation, error) {
	return c.mutateResource(ctx, companyID, http.MethodPut, resource, sourceID, idempotency, sourceVersion, actorType, actorID, session, payload)
}
func (c *Client) DeleteResource(ctx context.Context, companyID, resource, sourceID, idempotency string, sourceVersion int64, actorType, actorID, session string) (Operation, error) {
	return c.mutateResource(ctx, companyID, http.MethodDelete, resource, sourceID, idempotency, sourceVersion, actorType, actorID, session, nil)
}
func (c *Client) mutateResource(ctx context.Context, companyID, method, resource, sourceID, idempotency string, sourceVersion int64, actorType, actorID, session string, payload any) (Operation, error) {
	token, binding, e := c.tenantCredential(ctx, companyID)
	if e != nil {
		return Operation{}, e
	}
	path := "/integration/v1/" + resource
	if sourceID != "" {
		path += "/" + url.PathEscape(sourceID)
	}
	headers := map[string]string{"Source-Version": fmt.Sprint(sourceVersion), "BeeFiscal-Source-Actor-Type": actorType, "BeeFiscal-Source-Actor-Id": actorID}
	if session != "" {
		headers["BeeFiscal-Source-Actor-Session-Id"] = session
	}
	var out Operation
	e = c.call(ctx, method, path, token, idempotency, headers, payload, &out)
	if e != nil {
		return out, e
	}
	linkSourceID := sourceID
	if resource == "organization" && linkSourceID == "" {
		linkSourceID = "organization"
	}
	_, e = c.db.ExecContext(ctx, `insert into company_fiscal_resource_links(binding_id,resource_type,source_entity_id,source_version,sync_status,last_operation_id) values($1,$2,$3,$4,'ACCEPTED',$5) on conflict(binding_id,resource_type,source_entity_id) do update set source_version=excluded.source_version,sync_status='ACCEPTED',last_operation_id=excluded.last_operation_id,updated_at=now()`, binding, strings.TrimSuffix(resource, "s"), linkSourceID, sourceVersion, out.OperationID)
	return out, e
}

func (c *Client) UpdateSourceCompanyID(ctx context.Context, companyID, next string, expected int64, idempotency, actorType, actorID, session, reason string) error {
	token, _, e := c.tenantCredential(ctx, companyID)
	if e != nil {
		return e
	}
	headers := map[string]string{"BeeFiscal-Source-Actor-Type": actorType, "BeeFiscal-Source-Actor-Id": actorID}
	if session != "" {
		headers["BeeFiscal-Source-Actor-Session-Id"] = session
	}
	e = c.call(ctx, http.MethodPatch, "/integration/v1/binding", token, idempotency, headers, map[string]any{"source_company_id": next, "expected_version": expected}, nil)
	if e != nil {
		return e
	}
	tx, e := c.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var id string
	var before []byte
	e = tx.QueryRowContext(ctx, `select id::text,to_jsonb(b) from company_fiscal_bindings b where company_id=$1 and version=$2 for update`, companyID, expected).Scan(&id, &before)
	if errors.Is(e, sql.ErrNoRows) {
		return errors.New("binding version conflict")
	}
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `update company_fiscal_bindings set source_company_id=$2,version=version+1,updated_at=now() where id=$1`, id, next)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `insert into company_fiscal_binding_history(binding_id,actor_type,actor_id,reason,before_redacted,after_redacted) select id,$2,$3,$4,$5,to_jsonb(b) from company_fiscal_bindings b where id=$1`, id, actorType, actorID, reason, before)
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (c *Client) ProcessWebhook(ctx context.Context, eventID, systemID, tenantID, operationID, sourceID, status string, sourceVersion int64, result map[string]any) error {
	tx, e := c.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var bindingID string
	e = tx.QueryRowContext(ctx, `select id::text from company_fiscal_bindings where external_system_id=$1 and fiscal_tenant_id=$2 and status='ACTIVE'`, systemID, tenantID).Scan(&bindingID)
	if e != nil {
		return e
	}
	syncStatus := status
	if syncStatus != "SUCCEEDED" && syncStatus != "SUPERSEDED" {
		syncStatus = "FAILED"
	}
	resultJSON, _ := json.Marshal(result)
	_, e = tx.ExecContext(ctx, `update company_fiscal_resource_links set sync_status=$2,fiscal_resource_id=coalesce($3->>'id',fiscal_resource_id),fiscal_version=coalesce(($3->>'version')::bigint,fiscal_version),last_error_code=case when $2='FAILED' then 'FISCAL_OPERATION_FAILED' else null end,updated_at=now() where binding_id=$1 and source_entity_id=$4 and source_version=$5 and last_operation_id=$6`, bindingID, syncStatus, resultJSON, sourceID, sourceVersion, operationID)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `insert into minipos_runtime_webhook_inbox(event_id,organization_id,request_hash,received_at,payload) select $1,company_id,$2,now(),$3 from company_fiscal_bindings where id=$4 on conflict(event_id) do nothing`, eventID, operationID, resultJSON, bindingID)
	if e != nil {
		return e
	}
	return tx.Commit()
}
