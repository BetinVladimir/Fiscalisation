package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr, AppEnv, FiscalBaseURL, APIVersion, WebhookVerificationKey, DatabaseURL, RLSDatabaseURL, CORSAllowedOrigins, AuthHMACKey, RabbitMQURL, AuthTokenIssuer, FiscalAuthToken, OAuthTokenURL, OAuthClientID, OAuthClientSecret, OAuthScope, OAuthAudience string
	LocalTokenSigningKeyPEM, LocalTokenSigningKID, LocalTokenIssuer                                                                                                                                                                                              string
	EMQXBroker, EMQXClientID, EMQXUsername, EMQXToken                                                                                                                                                                                                            string
	EMQXSubTopics                                                                                                                                                                                                                                                []string
}

func Load() Config {
	return Config{HTTPAddr: get("HTTP_ADDR", ":8081"), AppEnv: get("APP_ENV", "dev"), FiscalBaseURL: get("FISCAL_PUBLIC_BASE_URL", "http://localhost:8080/public/v1"), APIVersion: get("API_VERSION", "2026-08-07"), WebhookVerificationKey: get("WEBHOOK_VERIFICATION_KEY", "dev-only-webhook-key"), DatabaseURL: os.Getenv("DATABASE_URL"), RLSDatabaseURL: os.Getenv("RLS_DATABASE_URL"), CORSAllowedOrigins: get("CORS_ALLOWED_ORIGINS", "*"), AuthHMACKey: get("AUTH_HMAC_KEY", "dev-only-minipos-auth-signing-key-32-bytes"), RabbitMQURL: get("RABBITMQ_URL", "amqp://fiscal:dev-only-rabbitmq-password@host.docker.internal:5672/"), AuthTokenIssuer: get("AUTH_TOKEN_ISSUER", "beeminipos"), FiscalAuthToken: os.Getenv("FISCAL_AUTH_TOKEN"), OAuthTokenURL: os.Getenv("FISCAL_OAUTH_TOKEN_URL"), OAuthClientID: os.Getenv("FISCAL_OAUTH_CLIENT_ID"), OAuthClientSecret: os.Getenv("FISCAL_OAUTH_CLIENT_SECRET"), OAuthScope: get("FISCAL_OAUTH_SCOPE", "fiscal.base"), OAuthAudience: os.Getenv("FISCAL_OAUTH_AUDIENCE"), LocalTokenSigningKeyPEM: os.Getenv("LOCAL_FISCAL_TOKEN_SIGNING_KEY_PEM"), LocalTokenSigningKID: get("LOCAL_FISCAL_TOKEN_SIGNING_KID", "minipos-local-1"), LocalTokenIssuer: get("LOCAL_FISCAL_TOKEN_ISSUER", "beeminipos"), EMQXBroker: os.Getenv("EMQX_BROKER"), EMQXClientID: get("EMQX_CLIENT_ID", "beeminipos-backend"), EMQXUsername: os.Getenv("EMQX_USERNAME"), EMQXToken: os.Getenv("EMQX_TOKEN"), EMQXSubTopics: splitCSV(os.Getenv("EMQX_SUB_TOPICS"))}
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
	if c.AppEnv == "prod" && !separateDatabaseUsers(c.DatabaseURL, c.RLSDatabaseURL) {
		return errors.New("DATABASE_URL and RLS_DATABASE_URL must use separate database users in PROD")
	}
	if c.AppEnv == "prod" && (len(c.AuthHMACKey) < 32 || strings.Contains(c.AuthHMACKey, "dev-")) {
		return errors.New("strong AUTH_HMAC_KEY required in PROD")
	}
	if c.AppEnv == "prod" && !strings.HasPrefix(c.RabbitMQURL, "amqps://") {
		return errors.New("RABBITMQ_URL must use amqps:// in PROD")
	}
	if c.AppEnv == "prod" && c.FiscalAuthToken != "" {
		return errors.New("FISCAL_AUTH_TOKEN forbidden in PROD; use client credentials")
	}
	if c.AppEnv == "prod" && (!httpsURL(c.OAuthTokenURL) || c.OAuthClientID == "" || len(c.OAuthClientSecret) < 16 || c.OAuthScope == "") {
		return errors.New("HTTPS Fiscal OAuth client credentials required in PROD")
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
