package emailauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db                                         *pgxpool.Pool
	secret                                     []byte
	issuer                                     string
	app                                        *domain.Service
	smtpHost, smtpUser, smtpPassword, smtpFrom string
	smtpPort                                   int
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type VerifyResult struct {
	OnboardingRequired bool   `json:"onboarding_required"`
	OnboardingToken    string `json:"onboarding_token,omitempty"`
	*Tokens
}

type Onboarding struct {
	CompanyName   string `json:"company_name"`
	Address       string `json:"address"`
	TaxIdentifier string `json:"tax_identifier"`
	FullName      string `json:"full_name"`
}

func New(ctx context.Context, databaseURL, signingKey, issuer string, app *domain.Service) (*Service, error) {
	if databaseURL == "" || len(signingKey) < 32 {
		return nil, errors.New("email auth requires DATABASE_URL and TOKEN_SIGNING_KEY (32+ bytes)")
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &Service{db: db, secret: []byte(signingKey), issuer: issuer, app: app}, nil
}

func (s *Service) ConfigureSMTP(host string, port int, user, password, from string) {
	s.smtpHost = host
	s.smtpPort = port
	s.smtpUser = user
	s.smtpPassword = password
	s.smtpFrom = from
}
func (s *Service) RunEmailWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.deliverOne(ctx)
		}
	}
}
func (s *Service) deliverOne(ctx context.Context) error {
	if s.smtpHost == "" || s.smtpFrom == "" {
		return nil
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var id, to, subject, body string
	e = tx.QueryRow(ctx, `with picked as (select id from minipos_email_outbox where status in ('PENDING','FAILED') and available_at<=now() and (lease_until is null or lease_until<now()) order by available_at,id for update skip locked limit 1) update minipos_email_outbox o set status='SENDING',lease_until=now()+interval '30 seconds',updated_at=now() from picked where o.id=picked.id returning o.id::text,o.recipient,o.subject,o.body_text`).Scan(&id, &to, &subject, &body)
	if errors.Is(e, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	message := []byte("From: " + s.smtpFrom + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	var auth smtp.Auth
	if s.smtpUser != "" {
		auth = smtp.PlainAuth("", s.smtpUser, s.smtpPassword, s.smtpHost)
	}
	e = smtp.SendMail(s.smtpHost+":"+strconv.Itoa(s.smtpPort), auth, s.smtpFrom, []string{to}, message)
	if e != nil {
		_, _ = s.db.Exec(ctx, `update minipos_email_outbox set status='FAILED',attempts=attempts+1,available_at=now()+least(interval '1 hour',interval '30 seconds'*(attempts+1)),lease_until=null,last_error=$2,updated_at=now() where id=$1`, id, e.Error())
		return e
	}
	_, e = s.db.Exec(ctx, `update minipos_email_outbox set status='SENT',sent_at=now(),lease_until=null,last_error=null,updated_at=now() where id=$1`, id)
	return e
}

func (s *Service) Close() { s.db.Close() }

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 {
		return "", errors.New("invalid email")
	}
	return value, nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func randomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func randomUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func (s *Service) RequestCode(ctx context.Context, email, language string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	var recent int
	if err = s.db.QueryRow(ctx, `select count(*) from minipos_auth_challenges where email=$1 and created_at>now()-interval '15 minutes'`, email).Scan(&recent); err != nil {
		return err
	}
	if recent >= 3 {
		return errors.New("too many code requests; try again later")
	}
	var b [4]byte
	if _, err = rand.Read(b[:]); err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", (uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))%1000000)
	challenge, err := randomUUID()
	if err != nil {
		return err
	}
	hash := digest(challenge + ":" + code + ":" + string(s.secret))
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `insert into minipos_auth_challenges(id,email,code_hash,expires_at) values($1,$2,$3,now()+interval '10 minutes')`, challenge, email, hash); err != nil {
		return err
	}
	subject := "MiniPOS sign-in code"
	body := "Your MiniPOS sign-in code is: " + code
	if language != "" {
		body += "\nLanguage: " + language
	}
	if _, err = tx.Exec(ctx, `insert into minipos_email_outbox(purpose,recipient,subject,body_text,status) values('MINIPOS_LOGIN_OTP',$1,$2,$3,'PENDING')`, email, subject, body); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) VerifyCode(ctx context.Context, email, code string) (VerifyResult, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return VerifyResult{}, err
	}
	var id, stored string
	err = s.db.QueryRow(ctx, `select id::text,code_hash from minipos_auth_challenges where email=$1 and consumed_at is null and expires_at>now() and attempts<5 order by created_at desc limit 1`, email).Scan(&id, &stored)
	if err != nil {
		return VerifyResult{}, errors.New("invalid or expired code")
	}
	if !hmac.Equal([]byte(stored), []byte(digest(id+":"+strings.TrimSpace(code)+":"+string(s.secret)))) {
		_, _ = s.db.Exec(ctx, `update minipos_auth_challenges set attempts=attempts+1 where id=$1`, id)
		return VerifyResult{}, errors.New("invalid or expired code")
	}
	_, _ = s.db.Exec(ctx, `update minipos_auth_challenges set consumed_at=now() where id=$1`, id)
	var tenant, employee string
	var roles []string
	err = s.db.QueryRow(ctx, `select organization_id::text,employee_id::text,roles from minipos_auth_accounts where email=$1`, email).Scan(&tenant, &employee, &roles)
	if errors.Is(err, pgx.ErrNoRows) {
		token, tokenErr := randomToken(32)
		if tokenErr != nil {
			return VerifyResult{}, tokenErr
		}
		_, tokenErr = s.db.Exec(ctx, `insert into minipos_auth_onboarding(token_hash,email,expires_at) values($1,$2,now()+interval '30 minutes')`, digest(token), email)
		return VerifyResult{OnboardingRequired: true, OnboardingToken: token}, tokenErr
	}
	if err != nil {
		return VerifyResult{}, err
	}
	tokens, err := s.issue(ctx, email, tenant, employee, roles)
	return VerifyResult{Tokens: &tokens}, err
}

func splitName(full string) (string, string, error) {
	parts := strings.Fields(full)
	if len(parts) == 0 {
		return "", "", errors.New("full name is required")
	}
	if len(parts) == 1 {
		return parts[0], "", nil
	}
	return parts[0], strings.Join(parts[1:], " "), nil
}

func (s *Service) Onboard(ctx context.Context, token string, input Onboarding) (Tokens, error) {
	first, last, err := splitName(input.FullName)
	if err != nil {
		return Tokens{}, err
	}
	if strings.TrimSpace(input.CompanyName) == "" || strings.TrimSpace(input.Address) == "" || strings.TrimSpace(input.TaxIdentifier) == "" {
		return Tokens{}, errors.New("all company fields are required")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback(ctx)
	var email string
	if err = tx.QueryRow(ctx, `select email from minipos_auth_onboarding where token_hash=$1 and consumed_at is null and expires_at>now() for update`, digest(token)).Scan(&email); err != nil {
		return Tokens{}, errors.New("invalid onboarding token")
	}
	var tenant string
	if err = tx.QueryRow(ctx, `insert into organizations(name,address,tax_identifier,fiscal_external_id,status) values($1,$2,$3,gen_random_uuid(),'ACTIVE') returning id::text`, strings.TrimSpace(input.CompanyName), strings.TrimSpace(input.Address), strings.TrimSpace(input.TaxIdentifier)).Scan(&tenant); err != nil {
		return Tokens{}, err
	}
	employee, err := s.app.CreateEmployee(domain.Employee{TenantID: tenant, FirstName: first, LastName: last, OperatorCode: "0001", Roles: []string{"ADMIN"}, Status: "ACTIVE"})
	if err != nil {
		return Tokens{}, err
	}
	if _, err = s.app.BindEmployeeIdentity(tenant, employee.ID, employee.ID, s.issuer); err != nil {
		return Tokens{}, err
	}
	if _, err = tx.Exec(ctx, `insert into minipos_auth_accounts(email,organization_id,employee_id,roles) values($1,$2,$3,array['ADMIN'])`, email, tenant, employee.ID); err != nil {
		return Tokens{}, err
	}
	if _, err = tx.Exec(ctx, `update minipos_auth_onboarding set consumed_at=now() where token_hash=$1`, digest(token)); err != nil {
		return Tokens{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, email, tenant, employee.ID, []string{"ADMIN"})
}

func (s *Service) Refresh(ctx context.Context, raw string) (Tokens, error) {
	hash := digest(raw)
	var email, tenant, employee string
	var roles []string
	err := s.db.QueryRow(ctx, `select a.email,a.organization_id::text,a.employee_id::text,a.roles from minipos_auth_refresh_tokens r join minipos_auth_accounts a on a.email=r.email where r.token_hash=$1 and r.revoked_at is null and r.expires_at>now()`, hash).Scan(&email, &tenant, &employee, &roles)
	if err != nil {
		return Tokens{}, errors.New("invalid refresh token")
	}
	_, _ = s.db.Exec(ctx, `update minipos_auth_refresh_tokens set revoked_at=now() where token_hash=$1`, hash)
	return s.issue(ctx, email, tenant, employee, roles)
}

func (s *Service) issue(ctx context.Context, email, tenant, employee string, roles []string) (Tokens, error) {
	expires := time.Now().Add(15 * time.Minute)
	access, err := auth.Sign(auth.Claims{Subject: employee, Issuer: s.issuer, TenantID: tenant, Roles: roles, Scope: "fiscal.base", ExpiresAt: expires.Unix()}, s.secret)
	if err != nil {
		return Tokens{}, err
	}
	refresh, err := randomToken(48)
	if err != nil {
		return Tokens{}, err
	}
	_, err = s.db.Exec(ctx, `insert into minipos_auth_refresh_tokens(token_hash,email,expires_at) values($1,$2,now()+interval '30 days')`, digest(refresh), email)
	return Tokens{AccessToken: access, RefreshToken: refresh, ExpiresIn: 900}, err
}
