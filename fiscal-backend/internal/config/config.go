package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr, AppEnv, PublicBaseURL, APIVersion, WebhookSigningKey, WebhookTargetURL, DatabaseURL, RLSDatabaseURL, CORSAllowedOrigins, AuthHMACKey, OIDCIssuer, OIDCAudience, OIDCJWKSURL, BLESigningKey string
	EMQXBroker, EMQXClientID, EMQXUsername, EMQXToken                                                                                                                                                    string
	EMQXSubTopics                                                                                                                                                                                        []string
	AllowStubAdapters, SimulatorCardTerminalAvailable                                                                                                                                                    bool
}

func Load() Config {
	return Config{HTTPAddr: getenv("HTTP_ADDR", ":8080"), AppEnv: getenv("APP_ENV", "dev"), PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://localhost:8080/public/v1"), APIVersion: getenv("API_VERSION", "2026-08-07"), WebhookSigningKey: getenv("WEBHOOK_SIGNING_KEY", "dev-only-webhook-key"), WebhookTargetURL: os.Getenv("WEBHOOK_TARGET_URL"), DatabaseURL: os.Getenv("DATABASE_URL"), RLSDatabaseURL: os.Getenv("RLS_DATABASE_URL"), CORSAllowedOrigins: getenv("CORS_ALLOWED_ORIGINS", "http://localhost:19006"), AuthHMACKey: os.Getenv("AUTH_HMAC_KEY"), OIDCIssuer: os.Getenv("OIDC_ISSUER"), OIDCAudience: os.Getenv("OIDC_AUDIENCE"), OIDCJWKSURL: os.Getenv("OIDC_JWKS_URL"), BLESigningKey: getenv("BLE_SIGNING_KEY", "dev-only-ble-signing-key-32-bytes"), EMQXBroker: os.Getenv("EMQX_BROKER"), EMQXClientID: getenv("EMQX_CLIENT_ID", "beefiscal-backend"), EMQXUsername: os.Getenv("EMQX_USERNAME"), EMQXToken: os.Getenv("EMQX_TOKEN"), EMQXSubTopics: splitCSV(os.Getenv("EMQX_SUB_TOPICS")), AllowStubAdapters: strings.EqualFold(getenv("ALLOW_STUB_ADAPTERS", "true"), "true"), SimulatorCardTerminalAvailable: strings.EqualFold(getenv("SIMULATOR_CARD_TERMINAL_AVAILABLE", "false"), "true")}
}
func (c Config) Validate() error {
	if c.AppEnv == "prod" && c.AllowStubAdapters {
		return errors.New("ALLOW_STUB_ADAPTERS must be false in PROD")
	}
	if c.AppEnv == "prod" && strings.Contains(c.WebhookSigningKey, "dev-") {
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
	if c.AppEnv == "prod" && (len(c.BLESigningKey) < 32 || strings.Contains(c.BLESigningKey, "dev-")) {
		return errors.New("strong BLE_SIGNING_KEY required in PROD")
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
func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
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
