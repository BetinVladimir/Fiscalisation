package integration

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrUnauthorized = errors.New("integration credential invalid")
	ErrConflict     = errors.New("integration conflict")
	ErrNotFound     = errors.New("integration record not found")
	ErrRateLimited  = errors.New("too many requests")
	codePattern     = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{1,62}$`)
	actorPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@/-]{0,254}$`)
)

type Service struct {
	db            *sql.DB
	pepper        []byte
	aead          cipher.AEAD
	now           func() time.Time
	appSigningKey []byte
}
type Principal struct {
	CredentialID, BindingID, TenantID, SystemID, SourceCompanyID string
	Scopes                                                       map[string]bool
}
type System struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	DisplayName   string    `json:"display_name"`
	Status        string    `json:"status"`
	WebhookURL    string    `json:"webhook_url"`
	WebhookEvents []string  `json:"webhook_events"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type EnrollmentRequest struct {
	Email           string `json:"email"`
	SourceCompanyID string `json:"source_company_id"`
	Company         struct {
		LegalName     string `json:"legal_name"`
		TaxIdentifier struct {
			Country string `json:"country"`
			Type    string `json:"type"`
			Value   string `json:"value"`
		} `json:"tax_identifier"`
		Address string `json:"address"`
	} `json:"company"`
}
type EnrollmentStart struct {
	TemporaryToken string    `json:"temporary_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	ResendAfter    time.Time `json:"resend_after"`
}
type EnrollmentResult struct {
	TenantID        string   `json:"tenant_id"`
	SourceSystemID  string   `json:"source_system_id"`
	SourceCompanyID string   `json:"source_company_id"`
	AccessToken     string   `json:"access_token"`
	TokenType       string   `json:"token_type"`
	Scopes          []string `json:"scopes"`
}
type CredentialRecoveryRequest struct {
	Email         string `json:"email"`
	TaxIdentifier struct {
		Country string `json:"country"`
		Type    string `json:"type"`
		Value   string `json:"value"`
	} `json:"tax_identifier"`
}
type AcceptedOperation struct {
	OperationID string    `json:"operation_id"`
	Status      string    `json:"status"`
	StatusURL   string    `json:"status_url"`
	AcceptedAt  time.Time `json:"accepted_at"`
}

func New(databaseURL string, pepper, encryptionKey []byte) (*Service, error) {
	if databaseURL == "" || len(pepper) < 32 || len(encryptionKey) != 32 {
		return nil, errors.New("integration database, 32-byte pepper and 32-byte encryption key required")
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(40)
	return &Service{db: db, pepper: pepper, aead: aead, now: func() time.Time { return time.Now().UTC() }}, nil
}
func (s *Service) Close() error    { return s.db.Close() }
func random(n int) ([]byte, error) { b := make([]byte, n); _, e := rand.Read(b); return b, e }
func uuid() (string, error) {
	b, e := random(16)
	if e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func (s *Service) digest(v string) []byte {
	h := hmac.New(sha256.New, s.pepper)
	h.Write([]byte(v))
	return h.Sum(nil)
}
func token(prefix, id string) (string, error) {
	b, e := random(32)
	if e != nil {
		return "", e
	}
	return prefix + "_" + id + "." + base64.RawURLEncoding.EncodeToString(b), nil
}
func splitToken(raw, prefix string) (string, bool) {
	p := strings.Split(raw, ".")
	if len(p) != 2 || !strings.HasPrefix(p[0], prefix+"_") {
		return "", false
	}
	return strings.TrimPrefix(p[0], prefix+"_"), true
}
func (s *Service) encrypt(v []byte) ([]byte, error) {
	n, e := random(s.aead.NonceSize())
	if e != nil {
		return nil, e
	}
	return s.aead.Seal(n, n, v, nil), nil
}
func (s *Service) decrypt(v []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(v) < n {
		return nil, errors.New("invalid ciphertext")
	}
	return s.aead.Open(nil, v[:n], v[n:], nil)
}
func normalizeTax(country, kind, value string) (string, string, string, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	kind = strings.ToUpper(strings.TrimSpace(kind))
	value = strings.ToUpper(strings.Join(strings.Fields(value), ""))
	value = strings.NewReplacer("-", "", ".", "", "/", "").Replace(value)
	if len(country) != 2 || !codePattern.MatchString(kind) || len(value) < 3 || len(value) > 64 {
		return "", "", "", errors.New("invalid tax identifier")
	}
	return country, kind, value, nil
}
func normalizeEmail(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if len(v) < 3 || len(v) > 254 || strings.Count(v, "@") != 1 {
		return "", errors.New("invalid email")
	}
	return v, nil
}
func validateSystemWebhook(raw string, events []string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("invalid HTTPS webhook URL")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("non-public webhook target")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return errors.New("non-public webhook target")
	}
	if len(events) == 0 {
		return errors.New("webhook events required")
	}
	seen := map[string]bool{}
	for _, v := range events {
		if v != "integration.command.updated" || seen[v] {
			return errors.New("unsupported or duplicate webhook event")
		}
		seen[v] = true
	}
	return nil
}

func (s *Service) CreateSystem(ctx context.Context, actor, requestID, code, name, webhook string, events []string) (System, string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !codePattern.MatchString(code) || strings.TrimSpace(name) == "" || validateSystemWebhook(webhook, events) != nil {
		return System{}, "", errors.New("invalid external system")
	}
	id, e := uuid()
	if e != nil {
		return System{}, "", e
	}
	secretToken, e := token("sys_live", id)
	if e != nil {
		return System{}, "", e
	}
	signingMAC := hmac.New(sha256.New, []byte(secretToken))
	signingMAC.Write([]byte("beefiscal-webhook-signing-v1"))
	signing := signingMAC.Sum(nil)
	enc, e := s.encrypt(signing)
	if e != nil {
		return System{}, "", e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return System{}, "", e
	}
	defer tx.Rollback()
	var out System
	var rawEvents []byte
	e = tx.QueryRowContext(ctx, `insert into external_systems(id,code,display_name,status,webhook_url,webhook_events,webhook_signing_secret_ciphertext,created_by,updated_by) values($1,$2,$3,'ACTIVE',$4,$5,$6,$7,$7) returning id::text,code,display_name,status,webhook_url,array_to_json(webhook_events),version,created_at,updated_at`, id, code, name, webhook, events, enc, actor).Scan(&out.ID, &out.Code, &out.DisplayName, &out.Status, &out.WebhookURL, &rawEvents, &out.Version, &out.CreatedAt, &out.UpdatedAt)
	if e != nil {
		return System{}, "", e
	}
	if e = json.Unmarshal(rawEvents, &out.WebhookEvents); e != nil {
		return System{}, "", e
	}
	_, e = tx.ExecContext(ctx, `insert into external_system_credentials(credential_id,external_system_id,secret_hash,key_fingerprint,status,created_by) values($1,$2,$3,$4,'ACTIVE',$5)`, id, id, s.digest(secretToken), hex.EncodeToString(s.digest(secretToken))[:16], actor)
	if e != nil {
		return System{}, "", e
	}
	_, e = tx.ExecContext(ctx, `insert into external_system_audit_log(external_system_id,action,actor_subject,request_id,after_redacted) values($1,'CREATED',$2,$3,$4)`, id, actor, requestID, mapJSON(out))
	if e != nil {
		return System{}, "", e
	}
	return out, secretToken, tx.Commit()
}
func mapJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func (s *Service) ListSystems(ctx context.Context) ([]System, error) {
	rows, e := s.db.QueryContext(ctx, `select id::text,code,display_name,status,webhook_url,array_to_json(webhook_events),version,created_at,updated_at from external_systems order by code`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []System
	for rows.Next() {
		var v System
		var rawEvents []byte
		if e = rows.Scan(&v.ID, &v.Code, &v.DisplayName, &v.Status, &v.WebhookURL, &rawEvents, &v.Version, &v.CreatedAt, &v.UpdatedAt); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(rawEvents, &v.WebhookEvents); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) RotateSystemKey(ctx context.Context, systemID, actor, requestID string) (string, error) {
	id, e := uuid()
	if e != nil {
		return "", e
	}
	raw, e := token("sys_live", id)
	if e != nil {
		return "", e
	}
	signingMAC := hmac.New(sha256.New, []byte(raw))
	signingMAC.Write([]byte("beefiscal-webhook-signing-v1"))
	signing, e := s.encrypt(signingMAC.Sum(nil))
	if e != nil {
		return "", e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, `update external_system_credentials set status='REVOKED',revoked_at=now(),revoked_by=$2,revoke_reason='rotation' where external_system_id=$1 and status='ACTIVE'`, systemID, actor)
	if e != nil {
		return "", e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return "", ErrNotFound
	}
	_, e = tx.ExecContext(ctx, `insert into external_system_credentials(credential_id,external_system_id,secret_hash,key_fingerprint,status,created_by) values($1,$2,$3,$4,'ACTIVE',$5)`, id, systemID, s.digest(raw), hex.EncodeToString(s.digest(raw))[:16], actor)
	if e != nil {
		return "", e
	}
	_, e = tx.ExecContext(ctx, `update external_systems set webhook_signing_secret_ciphertext=$2,version=version+1,updated_at=now(),updated_by=$3 where id=$1`, systemID, signing, actor)
	if e != nil {
		return "", e
	}
	_, e = tx.ExecContext(ctx, `insert into external_system_audit_log(external_system_id,action,actor_subject,request_id) values($1,'KEY_ROTATED',$2,$3)`, systemID, actor, requestID)
	if e != nil {
		return "", e
	}
	return raw, tx.Commit()
}
func (s *Service) SetSystemStatus(ctx context.Context, systemID, status, actor, requestID string) error {
	if status != "ACTIVE" && status != "SUSPENDED" && status != "REVOKED" {
		return errors.New("invalid status")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, `update external_systems set status=$2,version=version+1,updated_at=now(),updated_by=$3 where id=$1`, systemID, status, actor)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrNotFound
	}
	_, e = tx.ExecContext(ctx, `insert into external_system_audit_log(external_system_id,action,actor_subject,request_id,after_redacted) values($1,$2,$3,$4,jsonb_build_object('status',$2))`, systemID, status, actor, requestID)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) UpdateSystem(ctx context.Context, systemID, actor, requestID, name, webhook string, events []string, expected int64) (System, error) {
	if strings.TrimSpace(name) == "" || expected < 1 || validateSystemWebhook(webhook, events) != nil {
		return System{}, errors.New("invalid external system update")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return System{}, e
	}
	defer tx.Rollback()
	var before []byte
	e = tx.QueryRowContext(ctx, `select to_jsonb(s) from external_systems s where id=$1 for update`, systemID).Scan(&before)
	if e != nil {
		return System{}, e
	}
	var out System
	var rawEvents []byte
	e = tx.QueryRowContext(ctx, `update external_systems set display_name=$2,webhook_url=$3,webhook_events=$4,version=version+1,updated_at=now(),updated_by=$5 where id=$1 and version=$6 returning id::text,code,display_name,status,webhook_url,array_to_json(webhook_events),version,created_at,updated_at`, systemID, name, webhook, events, actor, expected).Scan(&out.ID, &out.Code, &out.DisplayName, &out.Status, &out.WebhookURL, &rawEvents, &out.Version, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return System{}, ErrConflict
	}
	if e != nil {
		return System{}, e
	}
	if e = json.Unmarshal(rawEvents, &out.WebhookEvents); e != nil {
		return System{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into external_system_audit_log(external_system_id,action,actor_subject,request_id,before_redacted,after_redacted) values($1,'UPDATED',$2,$3,$4,$5)`, systemID, actor, requestID, before, mapJSON(out))
	if e != nil {
		return System{}, e
	}
	return out, tx.Commit()
}

func (s *Service) SystemAudit(ctx context.Context, systemID string) ([]map[string]any, error) {
	return s.queryObjects(ctx, `select jsonb_build_object('id',id,'action',action,'actor_subject',actor_subject,'request_id',request_id,'before',before_redacted,'after',after_redacted,'occurred_at',occurred_at) from external_system_audit_log where external_system_id=$1 order by occurred_at desc limit 500`, systemID)
}
func (s *Service) SystemBindings(ctx context.Context, systemID string) ([]map[string]any, error) {
	return s.queryObjects(ctx, `select jsonb_build_object('id',id,'tenant_id',tenant_id,'source_company_id',source_company_id,'tax_country',tax_country,'tax_type',tax_type,'tax_identifier',tax_normalized_value,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at) from tenant_source_bindings where external_system_id=$1 order by created_at desc limit 500`, systemID)
}
func (s *Service) SystemDeliveries(ctx context.Context, systemID string) ([]map[string]any, error) {
	return s.queryObjects(ctx, `select jsonb_build_object('id',id,'event_id',event_id,'tenant_id',tenant_id,'event_type',event_type,'status',status,'attempts',attempts,'next_attempt_at',next_attempt_at,'last_http_status',last_http_status,'last_error_code',last_error_code,'last_error_detail',last_error_detail,'delivered_at',delivered_at,'created_at',created_at) from webhook_deliveries where external_system_id=$1 order by created_at desc limit 500`, systemID)
}
func (s *Service) queryObjects(ctx context.Context, query string, arg any) ([]map[string]any, error) {
	rows, e := s.db.QueryContext(ctx, query, arg)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var item map[string]any
		if e = json.Unmarshal(raw, &item); e != nil {
			return nil, e
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Service) RequeueDelivery(ctx context.Context, id, actor, requestID string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var systemID string
	e = tx.QueryRowContext(ctx, `update webhook_deliveries set status='RETRY',attempts=0,next_attempt_at=now(),lease_id=null,lease_until=null,updated_at=now() where id=$1 and status='DEAD' returning external_system_id::text`, id).Scan(&systemID)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrConflict
	}
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `insert into external_system_audit_log(external_system_id,action,actor_subject,request_id,after_redacted) values($1,'WEBHOOK_REQUEUED',$2,$3,jsonb_build_object('delivery_id',$4))`, systemID, actor, requestID, id)
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Service) authenticateSystem(ctx context.Context, raw string) (string, error) {
	id, ok := splitToken(raw, "sys_live")
	if !ok {
		return "", ErrUnauthorized
	}
	var systemID, status string
	var hash []byte
	e := s.db.QueryRowContext(ctx, `select c.external_system_id::text,c.secret_hash,s.status from external_system_credentials c join external_systems s on s.id=c.external_system_id where c.credential_id=$1 and c.status='ACTIVE'`, id).Scan(&systemID, &hash, &status)
	if e != nil || !hmac.Equal(hash, s.digest(raw)) || status != "ACTIVE" {
		return "", ErrUnauthorized
	}
	_, _ = s.db.ExecContext(ctx, `update external_system_credentials set last_used_at=now() where credential_id=$1`, id)
	return systemID, nil
}
func (s *Service) StartEnrollment(ctx context.Context, systemToken, idempotency, requestIP string, in EnrollmentRequest) (EnrollmentStart, error) {
	systemID, e := s.authenticateSystem(ctx, systemToken)
	if e != nil {
		return EnrollmentStart{}, e
	}
	email, e := normalizeEmail(in.Email)
	if e != nil {
		return EnrollmentStart{}, e
	}
	country, kind, tax, e := normalizeTax(in.Company.TaxIdentifier.Country, in.Company.TaxIdentifier.Type, in.Company.TaxIdentifier.Value)
	if e != nil {
		return EnrollmentStart{}, e
	}
	if in.SourceCompanyID == "" || in.Company.LegalName == "" || idempotency == "" {
		return EnrollmentStart{}, errors.New("missing enrollment fields")
	}
	var recent int
	if e = s.db.QueryRowContext(ctx, `select count(*) from external_enrollment_challenges where created_at>now()-interval '15 minutes' and (external_system_id=$1 or normalized_email=$2 or request_ip_hash=$3)`, systemID, email, s.digest(requestIP)).Scan(&recent); e != nil {
		return EnrollmentStart{}, e
	}
	if recent >= 10 {
		return EnrollmentStart{}, ErrRateLimited
	}
	payload := mapJSON(in)
	ph := sha256.Sum256(payload)
	var existingToken, existingHash []byte
	var exp time.Time
	e = s.db.QueryRowContext(ctx, `select response_ciphertext,expires_at,payload_hash from external_enrollment_challenges where external_system_id=$1 and idempotency_key=$2`, systemID, idempotency).Scan(&existingToken, &exp, &existingHash)
	if e == nil {
		if !hmac.Equal(existingHash, ph[:]) {
			return EnrollmentStart{}, ErrConflict
		}
		plain, de := s.decrypt(existingToken)
		if de != nil {
			return EnrollmentStart{}, de
		}
		var out EnrollmentStart
		if json.Unmarshal(plain, &out) != nil {
			return EnrollmentStart{}, errors.New("invalid replay")
		}
		return out, nil
	}
	if e != sql.ErrNoRows {
		return EnrollmentStart{}, e
	}
	id, e := uuid()
	if e != nil {
		return EnrollmentStart{}, e
	}
	temp, e := token("enroll_tmp", id)
	if e != nil {
		return EnrollmentStart{}, e
	}
	digits, e := random(4)
	if e != nil {
		return EnrollmentStart{}, e
	}
	otp := fmt.Sprintf("%06d", (uint32(digits[0])<<24|uint32(digits[1])<<16|uint32(digits[2])<<8|uint32(digits[3]))%1000000)
	now := s.now()
	out := EnrollmentStart{TemporaryToken: temp, ExpiresAt: now.Add(10 * time.Minute), ResendAfter: now.Add(time.Minute)}
	ciphertext, e := s.encrypt(mapJSON(out))
	if e != nil {
		return EnrollmentStart{}, e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return EnrollmentStart{}, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `insert into external_enrollment_challenges(id,external_system_id,source_company_id,normalized_email,tax_country,tax_type,tax_normalized_value,temporary_token_hash,otp_hash,payload_hash,legal_profile,idempotency_key,response_ciphertext,status,expires_at,request_ip_hash) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'PENDING',$14,$15)`, id, systemID, in.SourceCompanyID, email, country, kind, tax, s.digest(temp), s.digest(otp), ph[:], payload, idempotency, ciphertext, out.ExpiresAt, s.digest(requestIP))
	if e != nil {
		return EnrollmentStart{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into fiscal_email_outbox(purpose,recipient,subject,body_text) values('FISCAL_ENROLLMENT_OTP',$1,'Fiscal verification code',$2)`, email, "Your Fiscal verification code is: "+otp)
	if e != nil {
		return EnrollmentStart{}, e
	}
	return out, tx.Commit()
}
func (s *Service) VerifyEnrollment(ctx context.Context, tempToken, code, idempotency string) (EnrollmentResult, error) {
	id, ok := splitToken(tempToken, "enroll_tmp")
	if !ok || idempotency == "" {
		return EnrollmentResult{}, ErrUnauthorized
	}
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return EnrollmentResult{}, e
	}
	defer tx.Rollback()
	var systemID, sourceCompany, country, kind, tax, status string
	var tokenHash, otpHash, profile, replay []byte
	var attempts int
	var expires time.Time
	e = tx.QueryRowContext(ctx, `select external_system_id::text,source_company_id,tax_country,tax_type,tax_normalized_value,status,temporary_token_hash,otp_hash,legal_profile,response_ciphertext,attempts,expires_at from external_enrollment_challenges where id=$1 for update`, id).Scan(&systemID, &sourceCompany, &country, &kind, &tax, &status, &tokenHash, &otpHash, &profile, &replay, &attempts, &expires)
	if e != nil || !hmac.Equal(tokenHash, s.digest(tempToken)) {
		return EnrollmentResult{}, ErrUnauthorized
	}
	if status == "VERIFIED" {
		plain, de := s.decrypt(replay)
		if de != nil {
			return EnrollmentResult{}, de
		}
		var out EnrollmentResult
		if json.Unmarshal(plain, &out) != nil {
			return EnrollmentResult{}, errors.New("invalid verify replay")
		}
		return out, nil
	}
	if status != "PENDING" || s.now().After(expires) || attempts >= 5 {
		return EnrollmentResult{}, ErrUnauthorized
	}
	if !hmac.Equal(otpHash, s.digest(code)) {
		_, _ = tx.ExecContext(ctx, `update external_enrollment_challenges set attempts=attempts+1,status=case when attempts+1>=5 then 'LOCKED' else status end where id=$1`, id)
		_ = tx.Commit()
		return EnrollmentResult{}, ErrUnauthorized
	}
	_, e = tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, country+":"+kind+":"+tax)
	if e != nil {
		return EnrollmentResult{}, e
	}
	var duplicate bool
	e = tx.QueryRowContext(ctx, `select exists(select 1 from tenant_source_bindings where tax_country=$1 and tax_type=$2 and tax_normalized_value=$3 and status<>'REVOKED')`, country, kind, tax).Scan(&duplicate)
	if e != nil {
		return EnrollmentResult{}, e
	}
	if duplicate {
		_, _ = tx.ExecContext(ctx, `update external_enrollment_challenges set status='CONFLICT' where id=$1`, id)
		_ = tx.Commit()
		return EnrollmentResult{}, ErrConflict
	}
	tenantID, e := uuid()
	if e != nil {
		return EnrollmentResult{}, e
	}
	bindingID, e := uuid()
	if e != nil {
		return EnrollmentResult{}, e
	}
	credID, e := uuid()
	if e != nil {
		return EnrollmentResult{}, e
	}
	access, e := token("tenant_live", credID)
	if e != nil {
		return EnrollmentResult{}, e
	}
	scopes := []string{"organization.write", "locations.write", "registers.write", "operators.write", "operations.read"}
	_, e = tx.ExecContext(ctx, `insert into tenant_source_bindings(id,tenant_id,external_system_id,source_company_id,tax_country,tax_type,tax_normalized_value,source_metadata,status) values($1,$2,$3,$4,$5,$6,$7,$8,'ACTIVE')`, bindingID, tenantID, systemID, sourceCompany, country, kind, tax, profile)
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into tenant_integration_credentials(credential_id,binding_id,secret_hash,scopes,status,expires_at) values($1,$2,$3,$4,'ACTIVE',now()+interval '1 year')`, credID, bindingID, s.digest(access), scopes)
	if e != nil {
		return EnrollmentResult{}, e
	}
	var enrollmentEmail string
	if e = tx.QueryRowContext(ctx, `select normalized_email from external_enrollment_challenges where id=$1`, id).Scan(&enrollmentEmail); e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into tenant_user_memberships(tenant_id,normalized_email,roles,status) values($1,$2,array['ADMIN']::text[],'ACTIVE') on conflict(tenant_id,normalized_email) do update set status='ACTIVE',roles=array['ADMIN']::text[],updated_at=now()`, tenantID, enrollmentEmail)
	if e != nil {
		return EnrollmentResult{}, e
	}
	out := EnrollmentResult{TenantID: tenantID, SourceSystemID: systemID, SourceCompanyID: sourceCompany, AccessToken: access, TokenType: "Bearer", Scopes: scopes}
	enc, e := s.encrypt(mapJSON(out))
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `update external_enrollment_challenges set status='VERIFIED',verified_at=now(),tenant_id=$2,response_ciphertext=$3 where id=$1`, id, tenantID, enc)
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into integration_change_journal(tenant_id,external_system_id,authenticated_system_id,operation_id,idempotency_key,resource_type,source_entity_id,action,outcome,after_redacted) values($1,$2,$2,$3,$4,'organization',$5,'ENROLLMENT','APPLIED',$6)`, tenantID, systemID, id, idempotency, sourceCompany, profile)
	if e != nil {
		return EnrollmentResult{}, e
	}
	return out, tx.Commit()
}

// StartCredentialRecovery proves control of an existing tenant administrator's
// mailbox. It never reveals whether the email or tax identity exists.
func (s *Service) StartCredentialRecovery(ctx context.Context, systemToken, idempotency string, in CredentialRecoveryRequest) (EnrollmentStart, error) {
	systemID, e := s.authenticateSystem(ctx, systemToken)
	if e != nil {
		return EnrollmentStart{}, e
	}
	email, e := normalizeEmail(in.Email)
	if e != nil || idempotency == "" {
		return EnrollmentStart{}, errors.New("invalid recovery request")
	}
	country, kind, tax, e := normalizeTax(in.TaxIdentifier.Country, in.TaxIdentifier.Type, in.TaxIdentifier.Value)
	if e != nil {
		return EnrollmentStart{}, e
	}
	var bindingID, tenantID, sourceCompany string
	e = s.db.QueryRowContext(ctx, `select b.id::text,b.tenant_id::text,b.source_company_id from tenant_source_bindings b join tenant_user_memberships m on m.tenant_id=b.tenant_id and m.normalized_email=$5 and m.status='ACTIVE' where b.external_system_id=$1 and b.tax_country=$2 and b.tax_type=$3 and b.tax_normalized_value=$4 and b.status='ACTIVE'`, systemID, country, kind, tax, email).Scan(&bindingID, &tenantID, &sourceCompany)
	if e != nil {
		return EnrollmentStart{}, ErrUnauthorized
	}
	requestHash := sha256.Sum256(mapJSON(in))
	var replay, replayHash []byte
	e = s.db.QueryRowContext(ctx, `select response_ciphertext,payload_hash from external_enrollment_challenges where external_system_id=$1 and idempotency_key=$2`, systemID, idempotency).Scan(&replay, &replayHash)
	if e == nil {
		if !hmac.Equal(replayHash, requestHash[:]) {
			return EnrollmentStart{}, ErrConflict
		}
		plain, de := s.decrypt(replay)
		if de != nil {
			return EnrollmentStart{}, de
		}
		var out EnrollmentStart
		if json.Unmarshal(plain, &out) != nil {
			return EnrollmentStart{}, errors.New("invalid replay")
		}
		return out, nil
	}
	if e != sql.ErrNoRows {
		return EnrollmentStart{}, e
	}
	id, e := uuid()
	if e != nil {
		return EnrollmentStart{}, e
	}
	temp, e := token("recover_tmp", id)
	if e != nil {
		return EnrollmentStart{}, e
	}
	digits, e := random(4)
	if e != nil {
		return EnrollmentStart{}, e
	}
	otp := fmt.Sprintf("%06d", (uint32(digits[0])<<24|uint32(digits[1])<<16|uint32(digits[2])<<8|uint32(digits[3]))%1000000)
	now := s.now()
	out := EnrollmentStart{TemporaryToken: temp, ExpiresAt: now.Add(10 * time.Minute), ResendAfter: now.Add(time.Minute)}
	enc, e := s.encrypt(mapJSON(out))
	if e != nil {
		return EnrollmentStart{}, e
	}
	profile := mapJSON(map[string]string{"binding_id": bindingID, "tenant_id": tenantID})
	ph := requestHash
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return EnrollmentStart{}, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `insert into external_enrollment_challenges(id,external_system_id,source_company_id,normalized_email,tax_country,tax_type,tax_normalized_value,temporary_token_hash,otp_hash,payload_hash,legal_profile,idempotency_key,response_ciphertext,status,expires_at,purpose,tenant_id) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'PENDING',$14,'CREDENTIAL_RECOVERY',$15)`, id, systemID, sourceCompany, email, country, kind, tax, s.digest(temp), s.digest(otp), ph[:], profile, idempotency, enc, out.ExpiresAt, tenantID)
	if e != nil {
		return EnrollmentStart{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into fiscal_email_outbox(purpose,recipient,subject,body_text) values('FISCAL_CREDENTIAL_RECOVERY_OTP',$1,'Fiscal credential recovery code',$2)`, email, "Your Fiscal credential recovery code is: "+otp)
	if e != nil {
		return EnrollmentStart{}, e
	}
	return out, tx.Commit()
}

func (s *Service) VerifyCredentialRecovery(ctx context.Context, tempToken, code, idempotency string) (EnrollmentResult, error) {
	id, ok := splitToken(tempToken, "recover_tmp")
	if !ok || idempotency == "" {
		return EnrollmentResult{}, ErrUnauthorized
	}
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return EnrollmentResult{}, e
	}
	defer tx.Rollback()
	var systemID, sourceCompany, tenantID, bindingID, status, purpose string
	var tokenHash, otpHash, replay []byte
	var attempts int
	var expires time.Time
	e = tx.QueryRowContext(ctx, `select c.external_system_id::text,c.source_company_id,c.tenant_id::text,b.id::text,c.status,c.purpose,c.temporary_token_hash,c.otp_hash,c.response_ciphertext,c.attempts,c.expires_at from external_enrollment_challenges c join tenant_source_bindings b on b.tenant_id=c.tenant_id and b.external_system_id=c.external_system_id where c.id=$1 for update`, id).Scan(&systemID, &sourceCompany, &tenantID, &bindingID, &status, &purpose, &tokenHash, &otpHash, &replay, &attempts, &expires)
	if e != nil || purpose != "CREDENTIAL_RECOVERY" || !hmac.Equal(tokenHash, s.digest(tempToken)) {
		return EnrollmentResult{}, ErrUnauthorized
	}
	if status == "VERIFIED" {
		plain, de := s.decrypt(replay)
		if de != nil {
			return EnrollmentResult{}, de
		}
		var out EnrollmentResult
		if json.Unmarshal(plain, &out) != nil {
			return EnrollmentResult{}, errors.New("invalid replay")
		}
		return out, nil
	}
	if status != "PENDING" || s.now().After(expires) || attempts >= 5 {
		return EnrollmentResult{}, ErrUnauthorized
	}
	if !hmac.Equal(otpHash, s.digest(code)) {
		_, _ = tx.ExecContext(ctx, `update external_enrollment_challenges set attempts=attempts+1,status=case when attempts+1>=5 then 'LOCKED' else status end where id=$1`, id)
		_ = tx.Commit()
		return EnrollmentResult{}, ErrUnauthorized
	}
	credID, e := uuid()
	if e != nil {
		return EnrollmentResult{}, e
	}
	access, e := token("tenant_live", credID)
	if e != nil {
		return EnrollmentResult{}, e
	}
	scopes := []string{"organization.write", "locations.write", "registers.write", "operators.write", "operations.read"}
	_, e = tx.ExecContext(ctx, `update tenant_integration_credentials set status='REVOKED',revoked_at=now() where binding_id=$1 and status='ACTIVE'`, bindingID)
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into tenant_integration_credentials(credential_id,binding_id,secret_hash,scopes,status,expires_at) values($1,$2,$3,$4,'ACTIVE',now()+interval '1 year')`, credID, bindingID, s.digest(access), scopes)
	if e != nil {
		return EnrollmentResult{}, e
	}
	out := EnrollmentResult{TenantID: tenantID, SourceSystemID: systemID, SourceCompanyID: sourceCompany, AccessToken: access, TokenType: "Bearer", Scopes: scopes}
	enc, e := s.encrypt(mapJSON(out))
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `update external_enrollment_challenges set status='VERIFIED',verified_at=now(),response_ciphertext=$2 where id=$1`, id, enc)
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into integration_change_journal(tenant_id,external_system_id,authenticated_system_id,operation_id,idempotency_key,resource_type,source_entity_id,action,outcome,after_redacted) values($1,$2,$2,$3,$4,'credential',$5,'CREDENTIAL_RECOVERY','APPLIED',jsonb_build_object('credential_id',$6))`, tenantID, systemID, id, idempotency, sourceCompany, credID)
	if e != nil {
		return EnrollmentResult{}, e
	}
	return out, tx.Commit()
}

func (s *Service) AuthenticateTenant(ctx context.Context, raw string) (Principal, error) {
	id, ok := splitToken(raw, "tenant_live")
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	var p Principal
	var hash []byte
	var scopes []string
	var status string
	var expires time.Time
	var rawScopes []byte
	e := s.db.QueryRowContext(ctx, `select c.credential_id::text,c.binding_id::text,b.tenant_id::text,b.external_system_id::text,b.source_company_id,c.secret_hash,array_to_json(c.scopes),c.status,c.expires_at from tenant_integration_credentials c join tenant_source_bindings b on b.id=c.binding_id where c.credential_id=$1 and b.status='ACTIVE'`, id).Scan(&p.CredentialID, &p.BindingID, &p.TenantID, &p.SystemID, &p.SourceCompanyID, &hash, &rawScopes, &status, &expires)
	if e != nil || status != "ACTIVE" || s.now().After(expires) || !hmac.Equal(hash, s.digest(raw)) {
		return Principal{}, ErrUnauthorized
	}
	if json.Unmarshal(rawScopes, &scopes) != nil {
		return Principal{}, ErrUnauthorized
	}
	p.Scopes = map[string]bool{}
	for _, v := range scopes {
		p.Scopes[v] = true
	}
	_, _ = s.db.ExecContext(ctx, `update tenant_integration_credentials set last_used_at=now() where credential_id=$1`, id)
	return p, nil
}
func (s *Service) UpdateSourceCompanyID(ctx context.Context, p Principal, next string, expected int64, actorType, actorID, session, idempotency string) error {
	if strings.TrimSpace(next) == "" || expected < 1 || idempotency == "" || !actorPattern.MatchString(actorID) || (actorType != "USER" && actorType != "SERVICE") {
		return errors.New("invalid binding update")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var priorAfter []byte
	e = tx.QueryRowContext(ctx, `select after_redacted from integration_change_journal where tenant_id=$1 and external_system_id=$2 and idempotency_key=$3 and action='SOURCE_COMPANY_ID_UPDATED'`, p.TenantID, p.SystemID, idempotency).Scan(&priorAfter)
	if e == nil {
		var v map[string]string
		if json.Unmarshal(priorAfter, &v) == nil && v["source_company_id"] == next {
			return nil
		}
		return ErrConflict
	}
	if e != sql.ErrNoRows {
		return e
	}
	var previous string
	e = tx.QueryRowContext(ctx, `select source_company_id from tenant_source_bindings where id=$1 for update`, p.BindingID).Scan(&previous)
	if e != nil {
		return e
	}
	r, e := tx.ExecContext(ctx, `update tenant_source_bindings set source_company_id=$2,version=version+1,updated_at=now() where id=$1 and version=$3`, p.BindingID, next, expected)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	_, e = tx.ExecContext(ctx, `insert into integration_change_journal(tenant_id,external_system_id,authenticated_system_id,idempotency_key,resource_type,source_entity_id,action,outcome,before_redacted,after_redacted,asserted_actor_type,asserted_actor_id,asserted_actor_session_id) values($1,$2,$2,$3,'binding',$4,'SOURCE_COMPANY_ID_UPDATED','APPLIED',jsonb_build_object('source_company_id',$5),jsonb_build_object('source_company_id',$4),$6,$7,$8)`, p.TenantID, p.SystemID, idempotency, next, previous, actorType, actorID, session)
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Service) RotateTenantCredential(ctx context.Context, p Principal, idempotency string) (EnrollmentResult, error) {
	if idempotency == "" {
		return EnrollmentResult{}, errors.New("idempotency key required")
	}
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return EnrollmentResult{}, e
	}
	defer tx.Rollback()
	var replay []byte
	e = tx.QueryRowContext(ctx, `select response_ciphertext from integration_idempotency_replays where external_system_id=$1 and scope=$2 and idempotency_key=$3 and expires_at>now()`, p.SystemID, "credential-rotate:"+p.BindingID, idempotency).Scan(&replay)
	if e == nil {
		plain, de := s.decrypt(replay)
		if de != nil {
			return EnrollmentResult{}, de
		}
		var out EnrollmentResult
		if json.Unmarshal(plain, &out) != nil {
			return EnrollmentResult{}, errors.New("invalid replay")
		}
		return out, nil
	}
	if e != sql.ErrNoRows {
		return EnrollmentResult{}, e
	}
	credID, e := uuid()
	if e != nil {
		return EnrollmentResult{}, e
	}
	raw, e := token("tenant_live", credID)
	if e != nil {
		return EnrollmentResult{}, e
	}
	scopes := []string{}
	for v := range p.Scopes {
		scopes = append(scopes, v)
	}
	_, e = tx.ExecContext(ctx, `update tenant_integration_credentials set status='REVOKED',revoked_at=now() where binding_id=$1 and status='ACTIVE'`, p.BindingID)
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into tenant_integration_credentials(credential_id,binding_id,secret_hash,scopes,status,expires_at) values($1,$2,$3,$4,'ACTIVE',now()+interval '1 year')`, credID, p.BindingID, s.digest(raw), scopes)
	if e != nil {
		return EnrollmentResult{}, e
	}
	out := EnrollmentResult{TenantID: p.TenantID, SourceSystemID: p.SystemID, SourceCompanyID: p.SourceCompanyID, AccessToken: raw, TokenType: "Bearer", Scopes: scopes}
	enc, e := s.encrypt(mapJSON(out))
	if e != nil {
		return EnrollmentResult{}, e
	}
	h := sha256.Sum256([]byte(idempotency))
	_, e = tx.ExecContext(ctx, `insert into integration_idempotency_replays(external_system_id,scope,idempotency_key,request_hash,response_status,response_ciphertext,expires_at) values($1,$2,$3,$4,200,$5,now()+interval '24 hours')`, p.SystemID, "credential-rotate:"+p.BindingID, idempotency, h[:], enc)
	if e != nil {
		return EnrollmentResult{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into integration_change_journal(tenant_id,external_system_id,authenticated_system_id,idempotency_key,resource_type,source_entity_id,action,outcome,after_redacted) values($1,$2,$2,$3,'credential',$4,'CREDENTIAL_ROTATED','APPLIED',jsonb_build_object('credential_id',$5))`, p.TenantID, p.SystemID, idempotency, p.SourceCompanyID, credID)
	if e != nil {
		return EnrollmentResult{}, e
	}
	return out, tx.Commit()
}
func (s *Service) AcceptResource(ctx context.Context, p Principal, method, resourceType, sourceID, idempotency string, version int64, actorType, actorID, session string, payload []byte, baseURL string) (AcceptedOperation, error) {
	baseURL = strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/public/v1")
	if !p.Scopes[resourceType+"s.write"] && !p.Scopes[resourceType+".write"] {
		return AcceptedOperation{}, ErrUnauthorized
	}
	if idempotency == "" || version < 1 || !actorPattern.MatchString(actorID) || (actorType != "USER" && actorType != "SERVICE") {
		return AcceptedOperation{}, errors.New("invalid mutation metadata")
	}
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	var doc any
	if json.Unmarshal(payload, &doc) != nil {
		return AcceptedOperation{}, errors.New("invalid json")
	}
	hash := sha256.Sum256(append([]byte(method+":"+resourceType+":"+sourceID+":"+fmt.Sprint(version)+":"), payload...))
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return AcceptedOperation{}, e
	}
	defer tx.Rollback()
	var existing AcceptedOperation
	var oldHash []byte
	e = tx.QueryRowContext(ctx, `select id::text,status,created_at,payload_hash from integration_commands where external_system_id=$1 and tenant_id=$2 and idempotency_key=$3`, p.SystemID, p.TenantID, idempotency).Scan(&existing.OperationID, &existing.Status, &existing.AcceptedAt, &oldHash)
	if e == nil {
		if !hmac.Equal(oldHash, hash[:]) {
			return AcceptedOperation{}, ErrConflict
		}
		existing.StatusURL = strings.TrimRight(baseURL, "/") + "/integration/v1/operations/" + existing.OperationID
		return existing, nil
	}
	if e != sql.ErrNoRows {
		return AcceptedOperation{}, e
	}
	id, e := uuid()
	if e != nil {
		return AcceptedOperation{}, e
	}
	topic := "fiscal.integration." + resourceType
	_, e = tx.ExecContext(ctx, `insert into integration_commands(id,tenant_id,external_system_id,idempotency_key,http_method,resource_type,aggregate_source_id,source_version,payload,payload_hash,authenticated_system_id,asserted_actor_type,asserted_actor_id,asserted_actor_session_id,status) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$3,$11,$12,nullif($13,''),'ACCEPTED')`, id, p.TenantID, p.SystemID, idempotency, method, resourceType, sourceID, version, payload, hash[:], actorType, actorID, session)
	if e != nil {
		return AcceptedOperation{}, e
	}
	event := mapJSON(map[string]any{"command_id": id, "tenant_id": p.TenantID, "external_system_id": p.SystemID, "resource_type": resourceType, "source_entity_id": sourceID, "source_version": version})
	_, e = tx.ExecContext(ctx, `insert into integration_command_outbox(command_id,topic,payload,status) values($1,$2,$3,'PENDING')`, id, topic, event)
	if e != nil {
		return AcceptedOperation{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into integration_change_journal(tenant_id,external_system_id,authenticated_system_id,asserted_actor_type,asserted_actor_id,asserted_actor_session_id,operation_id,idempotency_key,resource_type,source_entity_id,action,outcome,after_redacted) values($1,$2,$2,$3,$4,nullif($5,''),$6,$7,$8,$9,$10,'ACCEPTED',$11)`, p.TenantID, p.SystemID, actorType, actorID, session, id, idempotency, resourceType, sourceID, method, payload)
	if e != nil {
		return AcceptedOperation{}, e
	}
	out := AcceptedOperation{OperationID: id, Status: "ACCEPTED", StatusURL: strings.TrimRight(baseURL, "/") + "/integration/v1/operations/" + id, AcceptedAt: s.now()}
	return out, tx.Commit()
}
func (s *Service) Operation(ctx context.Context, p Principal, id string) (map[string]any, error) {
	var status, resource, source string
	var version int64
	var result []byte
	var created, updated time.Time
	e := s.db.QueryRowContext(ctx, `select status,resource_type,aggregate_source_id,source_version,coalesce(result,'{}'),created_at,updated_at from integration_commands where id=$1 and tenant_id=$2 and external_system_id=$3`, id, p.TenantID, p.SystemID).Scan(&status, &resource, &source, &version, &result, &created, &updated)
	if e == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	var parsed any
	_ = json.Unmarshal(result, &parsed)
	return map[string]any{"operation_id": id, "status": status, "resource_type": resource, "source_entity_id": source, "source_version": version, "result": parsed, "accepted_at": created, "updated_at": updated}, nil
}
