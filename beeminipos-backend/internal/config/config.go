package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr, AppEnv, FiscalBaseURL, APIVersion, WebhookVerificationKey, DatabaseURL, RLSDatabaseURL, CORSAllowedOrigins, AuthHMACKey, OIDCIssuer, OIDCAudience, OIDCJWKSURL, FiscalAuthToken, OAuthTokenURL, OAuthClientID, OAuthClientSecret, OAuthScope, OAuthAudience string
	EMQXBroker, EMQXClientID, EMQXUsername, EMQXToken                                                                                                                                                                                                                     string
	EMQXSubTopics                                                                                                                                                                                                                                                         []string
}

func Load() Config {
	return Config{HTTPAddr: get("HTTP_ADDR", ":8081"), AppEnv: get("APP_ENV", "dev"), FiscalBaseURL: get("FISCAL_PUBLIC_BASE_URL", "http://localhost:8080/public/v1"), APIVersion: get("API_VERSION", "2026-08-07"), WebhookVerificationKey: get("WEBHOOK_VERIFICATION_KEY", "dev-only-webhook-key"), DatabaseURL: os.Getenv("DATABASE_URL"), RLSDatabaseURL: os.Getenv("RLS_DATABASE_URL"), CORSAllowedOrigins: get("CORS_ALLOWED_ORIGINS", "http://localhost:19006"), AuthHMACKey: os.Getenv("AUTH_HMAC_KEY"), OIDCIssuer: os.Getenv("OIDC_ISSUER"), OIDCAudience: os.Getenv("OIDC_AUDIENCE"), OIDCJWKSURL: os.Getenv("OIDC_JWKS_URL"), FiscalAuthToken: os.Getenv("FISCAL_AUTH_TOKEN"), OAuthTokenURL: os.Getenv("FISCAL_OAUTH_TOKEN_URL"), OAuthClientID: os.Getenv("FISCAL_OAUTH_CLIENT_ID"), OAuthClientSecret: os.Getenv("FISCAL_OAUTH_CLIENT_SECRET"), OAuthScope: get("FISCAL_OAUTH_SCOPE", "fiscal.base"), OAuthAudience: os.Getenv("FISCAL_OAUTH_AUDIENCE"), EMQXBroker: os.Getenv("EMQX_BROKER"), EMQXClientID: get("EMQX_CLIENT_ID", "beeminipos-backend"), EMQXUsername: os.Getenv("EMQX_USERNAME"), EMQXToken: os.Getenv("EMQX_TOKEN"), EMQXSubTopics: splitCSV(os.Getenv("EMQX_SUB_TOPICS"))}
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
	if strings.Contains(c.FiscalBaseURL, "postgres") || os.Getenv("FISCAL_DATABASE_URL") != "" {
		return errors.New("MiniPOS may use Fiscal only through public API")
	}
	if c.AppEnv == "prod" && strings.Contains(c.WebhookVerificationKey, "dev-") {
		return errors.New("development webhook key forbidden in PROD")
	}
	if c.AppEnv == "prod" && c.DatabaseURL == "" {
		return errors.New("DATABASE_URL required in PROD")
	}
	if c.AppEnv == "prod" && c.RLSDatabaseURL == "" {
		return errors.New("RLS_DATABASE_URL required in PROD")
	}
	if c.AppEnv == "prod" && !separateDatabaseUsers(c.DatabaseURL, c.RLSDatabaseURL) {
		return errors.New("DATABASE_URL and RLS_DATABASE_URL must use separate database users in PROD")
	}
	if c.AppEnv == "prod" && (c.OIDCIssuer == "" || c.OIDCAudience == "" || c.OIDCJWKSURL == "") {
		return errors.New("OIDC_ISSUER, OIDC_AUDIENCE and OIDC_JWKS_URL required in PROD")
	}
	if c.AppEnv == "prod" && (!httpsURL(c.OIDCIssuer) || !httpsURL(c.OIDCJWKSURL)) {
		return errors.New("OIDC issuer and JWKS URL must use HTTPS in PROD")
	}
	if c.AppEnv == "prod" && c.AuthHMACKey != "" {
		return errors.New("AUTH_HMAC_KEY forbidden in PROD; use OIDC RS256")
	}
	if c.AppEnv == "prod" && c.FiscalAuthToken != "" {
		return errors.New("FISCAL_AUTH_TOKEN forbidden in PROD; use client credentials")
	}
	if c.AppEnv == "prod" && (!httpsURL(c.OAuthTokenURL) || c.OAuthClientID == "" || len(c.OAuthClientSecret) < 16 || c.OAuthScope == "") {
		return errors.New("HTTPS Fiscal OAuth client credentials required in PROD")
	}
	return nil
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
func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
