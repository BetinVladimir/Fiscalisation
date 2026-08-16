package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"fiscalisation/fiscal-backend/internal/auth"
)

type AppTenant struct {
	TenantID    string   `json:"tenant_id"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}
type AppChallenge struct {
	TemporaryToken string    `json:"temporary_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	ResendAfter    time.Time `json:"resend_after"`
}
type AppSession struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	Tenant       AppTenant `json:"tenant"`
}
type AppVerification struct {
	SelectionRequired    bool        `json:"selection_required"`
	TenantSelectionToken string      `json:"tenant_selection_token,omitempty"`
	ExpiresAt            time.Time   `json:"expires_at,omitempty"`
	Tenants              []AppTenant `json:"tenants,omitempty"`
	Session              *AppSession `json:"session,omitempty"`
}

func (s *Service) SetAppSigningKey(key []byte) { s.appSigningKey = append([]byte(nil), key...) }
func (s *Service) StartAppChallenge(ctx context.Context, email, appInstance, ip string) (AppChallenge, error) {
	normalized, e := normalizeEmail(email)
	if e != nil {
		return AppChallenge{}, e
	}
	if appInstance == "" {
		return AppChallenge{}, errors.New("app_instance_id required")
	}
	var recent int
	if e = s.db.QueryRowContext(ctx, `select count(*) from app_auth_challenges where created_at>now()-interval '15 minutes' and (normalized_email=$1 or app_instance_id::text=$2 or request_ip_hash=$3)`, normalized, appInstance, s.digest(ip)).Scan(&recent); e != nil {
		return AppChallenge{}, e
	}
	if recent >= 10 {
		return AppChallenge{}, ErrRateLimited
	}
	id, e := uuid()
	if e != nil {
		return AppChallenge{}, e
	}
	temporary, e := token("app_tmp", id)
	if e != nil {
		return AppChallenge{}, e
	}
	now := s.now()
	out := AppChallenge{TemporaryToken: temporary, ExpiresAt: now.Add(10 * time.Minute), ResendAfter: now.Add(time.Minute)}
	var known bool
	e = s.db.QueryRowContext(ctx, `select exists(select 1 from tenant_user_memberships where normalized_email=$1 and status='ACTIVE')`, normalized).Scan(&known)
	if e != nil {
		return out, e
	}
	status := "UNKNOWN_EMAIL"
	var otpHash any
	var otp string
	if known {
		status = "PENDING"
		b, _ := random(4)
		otp = fmtSix(b)
		otpHash = s.digest(otp)
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `insert into app_auth_challenges(id,normalized_email,temporary_token_hash,otp_hash,status,expires_at,request_ip_hash,app_instance_id) values($1,$2,$3,$4,$5,$6,$7,$8)`, id, normalized, s.digest(temporary), otpHash, status, out.ExpiresAt, s.digest(ip), appInstance)
	if e != nil {
		return out, e
	}
	if known {
		_, e = tx.ExecContext(ctx, `insert into fiscal_email_outbox(purpose,recipient,subject,body_text) values('BEEFISCAL_APP_OTP',$1,'BeeFiscal sign-in code',$2)`, normalized, "Your BeeFiscal sign-in code is: "+otp)
		if e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
func fmtSix(b []byte) string {
	if len(b) < 4 {
		return "000000"
	}
	n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1000000
	digits := "000000" + strconv.FormatUint(uint64(n), 10)
	return digits[len(digits)-6:]
}
func (s *Service) appTenants(ctx context.Context, email string) ([]AppTenant, error) {
	return appTenantsQuery(ctx, s.db, email)
}

type appQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func appTenantsQuery(ctx context.Context, q appQuerier, email string) ([]AppTenant, error) {
	rows, e := q.QueryContext(ctx, `select m.tenant_id::text,coalesce((b.source_metadata->'company'->>'legal_name'),(b.source_metadata->>'legal_name'),m.tenant_id::text),array_to_json(m.roles) from tenant_user_memberships m left join tenant_source_bindings b on b.tenant_id=m.tenant_id where m.normalized_email=$1 and m.status='ACTIVE' order by 2`, email)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []AppTenant
	for rows.Next() {
		var v AppTenant
		var rawRoles []byte
		if e = rows.Scan(&v.TenantID, &v.DisplayName, &rawRoles); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(rawRoles, &v.Roles); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) VerifyAppChallenge(ctx context.Context, temporary, code, appInstance string) (AppVerification, error) {
	id, ok := splitToken(temporary, "app_tmp")
	if !ok {
		return AppVerification{}, ErrUnauthorized
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return AppVerification{}, e
	}
	defer tx.Rollback()
	var email, status, instance string
	var th, oh []byte
	var attempts int
	var expires time.Time
	e = tx.QueryRowContext(ctx, `select normalized_email,status,app_instance_id::text,temporary_token_hash,otp_hash,attempts,expires_at from app_auth_challenges where id=$1 for update`, id).Scan(&email, &status, &instance, &th, &oh, &attempts, &expires)
	if e != nil || instance != appInstance || status != "PENDING" || s.now().After(expires) || !hmac.Equal(th, s.digest(temporary)) {
		return AppVerification{}, ErrUnauthorized
	}
	if !hmac.Equal(oh, s.digest(code)) {
		_, _ = tx.ExecContext(ctx, `update app_auth_challenges set attempts=attempts+1,status=case when attempts+1>=5 then 'LOCKED' else status end where id=$1`, id)
		_ = tx.Commit()
		return AppVerification{}, ErrUnauthorized
	}
	tenants, e := appTenantsQuery(ctx, tx, email)
	if e != nil {
		return AppVerification{}, e
	}
	if len(tenants) == 0 {
		return AppVerification{}, ErrUnauthorized
	}
	if len(tenants) == 1 {
		_, e = tx.ExecContext(ctx, `update app_auth_challenges set status='CANCELLED',verified_at=now() where id=$1 and status='PENDING'`, id)
		if e != nil {
			return AppVerification{}, e
		}
		session, e := s.createAppSessionTx(ctx, tx, email, appInstance, tenants[0])
		if e != nil {
			return AppVerification{}, e
		}
		if e = tx.Commit(); e != nil {
			return AppVerification{}, e
		}
		return AppVerification{Session: &session}, nil
	}
	_, e = tx.ExecContext(ctx, `update app_auth_challenges set status='VERIFIED',verified_at=now() where id=$1 and status='PENDING'`, id)
	if e != nil {
		return AppVerification{}, e
	}
	if e = tx.Commit(); e != nil {
		return AppVerification{}, e
	}
	return AppVerification{SelectionRequired: true, TenantSelectionToken: temporary, ExpiresAt: expires, Tenants: tenants}, nil
}
func (s *Service) SelectAppTenant(ctx context.Context, selection, tenantID, appInstance string) (AppSession, error) {
	id, ok := splitToken(selection, "app_tmp")
	if !ok {
		return AppSession{}, ErrUnauthorized
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return AppSession{}, e
	}
	defer tx.Rollback()
	var email, status, instance string
	var hash []byte
	var expires time.Time
	e = tx.QueryRowContext(ctx, `select normalized_email,status,app_instance_id::text,temporary_token_hash,expires_at from app_auth_challenges where id=$1 for update`, id).Scan(&email, &status, &instance, &hash, &expires)
	if e != nil || status != "VERIFIED" || instance != appInstance || s.now().After(expires) || !hmac.Equal(hash, s.digest(selection)) {
		return AppSession{}, ErrUnauthorized
	}
	tenants, e := appTenantsQuery(ctx, tx, email)
	if e != nil {
		return AppSession{}, e
	}
	for _, v := range tenants {
		if v.TenantID == tenantID {
			session, e := s.createAppSessionTx(ctx, tx, email, appInstance, v)
			if e != nil {
				return AppSession{}, e
			}
			result, e := tx.ExecContext(ctx, `update app_auth_challenges set status='CANCELLED' where id=$1 and status='VERIFIED'`, id)
			if e != nil {
				return AppSession{}, e
			}
			if affected, _ := result.RowsAffected(); affected != 1 {
				return AppSession{}, ErrUnauthorized
			}
			if e = tx.Commit(); e != nil {
				return AppSession{}, e
			}
			return session, nil
		}
	}
	return AppSession{}, ErrUnauthorized
}
func (s *Service) createAppSession(ctx context.Context, email, instance string, tenant AppTenant) (AppSession, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return AppSession{}, e
	}
	defer tx.Rollback()
	session, e := s.createAppSessionTx(ctx, tx, email, instance, tenant)
	if e != nil {
		return AppSession{}, e
	}
	return session, tx.Commit()
}

func (s *Service) createAppSessionTx(ctx context.Context, tx *sql.Tx, email, instance string, tenant AppTenant) (AppSession, error) {
	if len(s.appSigningKey) < 32 {
		return AppSession{}, errors.New("app token signing key unavailable")
	}
	sessionID, e := uuid()
	if e != nil {
		return AppSession{}, e
	}
	jti, e := uuid()
	if e != nil {
		return AppSession{}, e
	}
	refreshID, e := uuid()
	if e != nil {
		return AppSession{}, e
	}
	refresh, e := token("app_refresh", refreshID)
	if e != nil {
		return AppSession{}, e
	}
	var userID string
	e = tx.QueryRowContext(ctx, `select user_id::text from tenant_user_memberships where tenant_id=$1 and normalized_email=$2 and status='ACTIVE' for key share`, tenant.TenantID, email).Scan(&userID)
	if e != nil {
		return AppSession{}, e
	}
	now := s.now()
	expires := now.Add(15 * time.Minute)
	access := signAppJWT(s.appSigningKey, map[string]any{"sub": userID, "iss": "beefiscal-app", "tenant_id": tenant.TenantID, "roles": tenant.Roles, "scope": "fiscal.base", "jti": jti, "iat": now.Unix(), "exp": expires.Unix()})
	_, e = tx.ExecContext(ctx, `insert into app_auth_sessions(id,user_id,normalized_email,selected_tenant_id,refresh_token_hash,app_instance_id,status,expires_at) values($1,$2,$3,$4,$5,$6,'ACTIVE',now()+interval '30 days')`, sessionID, userID, email, tenant.TenantID, s.digest(refresh), instance)
	if e != nil {
		return AppSession{}, e
	}
	_, e = tx.ExecContext(ctx, `insert into app_issued_tokens(jti,session_id,tenant_id,issued_at,expires_at,status) values($1,$2,$3,$4,$5,'ACTIVE')`, jti, sessionID, tenant.TenantID, now, expires)
	if e != nil {
		return AppSession{}, e
	}
	return AppSession{AccessToken: access, RefreshToken: refresh, ExpiresIn: 900, Tenant: tenant}, nil
}
func signAppJWT(key []byte, claims map[string]any) string {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(body)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(head + "." + payload))
	return head + "." + payload + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func (s *Service) IsAppTokenRevoked(ctx context.Context, jti string) bool {
	if jti == "" {
		return false
	}
	var active bool
	e := s.db.QueryRowContext(ctx, `select exists(select 1 from app_issued_tokens t join app_auth_sessions s on s.id=t.session_id join tenant_user_memberships m on m.tenant_id=t.tenant_id and m.normalized_email=s.normalized_email where t.jti=$1 and t.status='ACTIVE' and t.expires_at>now() and s.status='ACTIVE' and s.expires_at>now() and m.status='ACTIVE')`, jti).Scan(&active)
	return e != nil || !active
}

func (s *Service) AppTenantsForAccess(ctx context.Context, raw string) ([]AppTenant, error) {
	claims, e := auth.Parse(raw, s.appSigningKey, s.now())
	if e != nil || claims.JTI == "" || s.IsAppTokenRevoked(ctx, claims.JTI) {
		return nil, ErrUnauthorized
	}
	var email string
	e = s.db.QueryRowContext(ctx, `select s.normalized_email from app_issued_tokens t join app_auth_sessions s on s.id=t.session_id where t.jti=$1`, claims.JTI).Scan(&email)
	if e != nil {
		return nil, ErrUnauthorized
	}
	return s.appTenants(ctx, email)
}
func (s *Service) RevokeAppToken(ctx context.Context, jti, reason string) error {
	_, e := s.db.ExecContext(ctx, `update app_issued_tokens set status='REVOKED',revoked_at=now(),revoke_reason=$2 where jti=$1 and status='ACTIVE'`, jti, reason)
	return e
}

func (s *Service) appSessionByRefreshTx(ctx context.Context, tx *sql.Tx, raw, instance string) (string, string, string, error) {
	var sessionID, email, tenant string
	e := tx.QueryRowContext(ctx, `select id::text,normalized_email,selected_tenant_id::text from app_auth_sessions where refresh_token_hash=$1 and app_instance_id=$2 and status='ACTIVE' and expires_at>now() for update`, s.digest(raw), instance).Scan(&sessionID, &email, &tenant)
	if e != nil {
		return "", "", "", ErrUnauthorized
	}
	return sessionID, email, tenant, nil
}
func (s *Service) RotateAppSession(ctx context.Context, refresh, instance, targetTenant string) (AppSession, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return AppSession{}, e
	}
	defer tx.Rollback()
	sessionID, email, current, e := s.appSessionByRefreshTx(ctx, tx, refresh, instance)
	if e != nil {
		return AppSession{}, e
	}
	if targetTenant == "" {
		targetTenant = current
	}
	tenants, e := appTenantsQuery(ctx, tx, email)
	if e != nil {
		return AppSession{}, e
	}
	var selected *AppTenant
	for i := range tenants {
		if tenants[i].TenantID == targetTenant {
			selected = &tenants[i]
			break
		}
	}
	if selected == nil {
		return AppSession{}, ErrUnauthorized
	}
	newSession, e := s.createAppSessionTx(ctx, tx, email, instance, *selected)
	if e != nil {
		return AppSession{}, e
	}
	_, e = tx.ExecContext(ctx, `update app_issued_tokens set status='REVOKED',revoked_at=now(),revoke_reason=$2 where session_id=$1 and status='ACTIVE'`, sessionID, "SESSION_ROTATED")
	if e != nil {
		return AppSession{}, e
	}
	_, e = tx.ExecContext(ctx, `update app_auth_sessions set status='REVOKED',revoked_at=now() where id=$1 and status='ACTIVE'`, sessionID)
	if e != nil {
		return AppSession{}, e
	}
	if e = tx.Commit(); e != nil {
		return AppSession{}, e
	}
	return newSession, nil
}
func (s *Service) LogoutAppSession(ctx context.Context, refresh, instance string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	sessionID, _, _, e := s.appSessionByRefreshTx(ctx, tx, refresh, instance)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `update app_issued_tokens set status='REVOKED',revoked_at=now(),revoke_reason='LOGOUT' where session_id=$1 and status='ACTIVE'`, sessionID)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `update app_auth_sessions set status='REVOKED',revoked_at=now() where id=$1`, sessionID)
	if e != nil {
		return e
	}
	return tx.Commit()
}
