package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr, AppEnv, FiscalBaseURL, APIVersion, WebhookVerificationKey, DatabaseURL, RLSDatabaseURL, CORSAllowedOrigins, AuthHMACKey, AuthTokenIssuer, FiscalAuthToken, OAuthTokenURL, OAuthClientID, OAuthClientSecret, OAuthScope, OAuthAudience string
	LocalTokenSigningKeyPEM, LocalTokenSigningKID, LocalTokenIssuer                                                                                                                                                                                 string
	FiscalSystemToken, FiscalCredentialEncryptionKeyBase64, FiscalCredentialKEKID                                                                                                                                                                   string
	SMTPHost, SMTPUser, SMTPPassword, SMTPFrom                                                                                                                                                                                                      string
	SMTPPort                                                                                                                                                                                                                                        int
	EMQXBroker, EMQXClientID, EMQXUsername, EMQXToken                                                                                                                                                                                               string
	EMQXSubTopics                                                                                                                                                                                                                                   []string
}

func Load() Config {
	port, _ := strconv.Atoi(get("SMTP_PORT", "587"))
	return Config{HTTPAddr: get("HTTP_ADDR", ":8081"), AppEnv: get("APP_ENV", "dev"), FiscalBaseURL: get("FISCAL_PUBLIC_BASE_URL", "http://localhost:8080/public/v1"), APIVersion: get("API_VERSION", "2026-08-07"), WebhookVerificationKey: os.Getenv("WEBHOOK_VERIFICATION_KEY"), DatabaseURL: os.Getenv("DATABASE_URL"), RLSDatabaseURL: os.Getenv("RLS_DATABASE_URL"), CORSAllowedOrigins: get("CORS_ALLOWED_ORIGINS", "*"), AuthHMACKey: os.Getenv("AUTH_HMAC_KEY"), AuthTokenIssuer: get("AUTH_TOKEN_ISSUER", "beeminipos"), FiscalAuthToken: os.Getenv("FISCAL_AUTH_TOKEN"), OAuthTokenURL: os.Getenv("FISCAL_OAUTH_TOKEN_URL"), OAuthClientID: os.Getenv("FISCAL_OAUTH_CLIENT_ID"), OAuthClientSecret: os.Getenv("FISCAL_OAUTH_CLIENT_SECRET"), OAuthScope: get("FISCAL_OAUTH_SCOPE", "fiscal.base"), OAuthAudience: os.Getenv("FISCAL_OAUTH_AUDIENCE"), LocalTokenSigningKeyPEM: os.Getenv("LOCAL_FISCAL_TOKEN_SIGNING_KEY_PEM"), LocalTokenSigningKID: get("LOCAL_FISCAL_TOKEN_SIGNING_KID", "minipos-local-1"), LocalTokenIssuer: get("LOCAL_FISCAL_TOKEN_ISSUER", "beeminipos"), FiscalSystemToken: os.Getenv("FISCAL_SYSTEM_TOKEN"), FiscalCredentialEncryptionKeyBase64: os.Getenv("FISCAL_CREDENTIAL_ENCRYPTION_KEY_BASE64"), FiscalCredentialKEKID: get("FISCAL_CREDENTIAL_KEK_ID", "local-dev-key"), SMTPHost: os.Getenv("SMTP_HOST"), SMTPUser: os.Getenv("SMTP_USER"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: os.Getenv("SMTP_FROM"), SMTPPort: port, EMQXBroker: os.Getenv("EMQX_BROKER"), EMQXClientID: get("EMQX_CLIENT_ID", "beeminipos-backend"), EMQXUsername: os.Getenv("EMQX_USERNAME"), EMQXToken: os.Getenv("EMQX_TOKEN"), EMQXSubTopics: splitCSV(os.Getenv("EMQX_SUB_TOPICS"))}
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
func (c Config) Validate() error {
	if !publicFiscalBaseURL(c.FiscalBaseURL) || os.Getenv("FISCAL_DATABASE_URL") != "" {
		return errors.New("MiniPOS may use Fiscal only through public API")
	}
	if c.AppEnv == "prod" && strings.Contains(c.WebhookVerificationKey, "dev-") {
		return errors.New("development webhook key forbidden in PROD")
	}
	if c.AppEnv == "prod" && len(c.WebhookVerificationKey) < 32 {
		return errors.New("WEBHOOK_VERIFICATION_KEY must contain at least 32 bytes in PROD")
	}
	if c.AppEnv == "prod" && !httpsURL(c.FiscalBaseURL) {
		return errors.New("FISCAL_PUBLIC_BASE_URL must use HTTPS in PROD")
	}
	if c.AppEnv == "prod" && c.CORSAllowedOrigins != "*" && !secureOrigins(c.CORSAllowedOrigins) {
		return errors.New("CORS_ALLOWED_ORIGINS must be * or an explicit HTTPS origin list in PROD")
	}
	if c.AppEnv == "prod" && c.DatabaseURL == "" {
		return errors.New("DATABASE_URL required in PROD")
	}
	if c.AppEnv == "prod" && c.RLSDatabaseURL == "" {
		return errors.New("RLS_DATABASE_URL required in PROD")
	}
	if c.AppEnv == "prod" && (strings.Contains(c.DatabaseURL, "sslmode=disable") || strings.Contains(c.RLSDatabaseURL, "sslmode=disable")) {
		return errors.New("sslmode=disable is not permitted in PROD — use sslmode=require or verify-full")
	}
	if c.AppEnv == "prod" && !separateDatabaseUsers(c.DatabaseURL, c.RLSDatabaseURL) {
		return errors.New("DATABASE_URL and RLS_DATABASE_URL must use separate database users in PROD")
	}
	if c.AppEnv == "prod" && (len(c.AuthHMACKey) < 32 || strings.Contains(c.AuthHMACKey, "dev-")) {
		return errors.New("strong AUTH_HMAC_KEY required in PROD")
	}
	if c.AppEnv == "prod" && c.FiscalAuthToken != "" {
		return errors.New("FISCAL_AUTH_TOKEN forbidden in PROD; use client credentials")
	}
	if c.AppEnv == "prod" && (!httpsURL(c.OAuthTokenURL) || c.OAuthClientID == "" || len(c.OAuthClientSecret) < 16 || c.OAuthScope == "") {
		if c.FiscalSystemToken == "" {
			return errors.New("FISCAL_SYSTEM_TOKEN required in PROD during OAuth migration")
		}
	}
	if c.AppEnv == "prod" && (c.FiscalSystemToken == "" || c.FiscalCredentialEncryptionKeyBase64 == "" || c.FiscalCredentialKEKID == "") {
		return errors.New("Fiscal system token and credential encryption configuration required in PROD")
	}
	if c.AppEnv == "prod" && (c.SMTPHost == "" || c.SMTPFrom == "" || c.SMTPPort < 1) {
		return errors.New("MiniPOS-owned SMTP configuration required in PROD")
	}
	if c.AppEnv == "prod" && (c.LocalTokenSigningKeyPEM == "" || c.LocalTokenSigningKID == "" || c.LocalTokenIssuer == "") {
		return errors.New("local fiscal ES256 token signer required in PROD")
	}
	return nil
}
func publicFiscalBaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Path == "/public/v1" && parsed.RawPath == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}
func httpsURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func separateDatabaseUsers(writeURL, readURL string) bool {
	write, writeErr := url.Parse(writeURL)
	read, readErr := url.Parse(readURL)
	return writeErr == nil && readErr == nil && write.User != nil && read.User != nil &&
		write.User.Username() != "" && read.User.Username() != "" && write.User.Username() != read.User.Username()
}
func secureOrigins(raw string) bool {
	values := splitCSV(raw)
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return false
		}
	}
	return true
}
func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
