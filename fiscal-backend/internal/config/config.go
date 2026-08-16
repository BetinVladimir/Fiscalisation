package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr, AppEnv, PublicBaseURL, APIVersion, DatabaseURL, RLSDatabaseURL, CORSAllowedOrigins, AuthHMACKey, OIDCIssuer, OIDCAudience, OIDCJWKSURL, BLESigningKey string
	RabbitMQURL, SMTPHost, SMTPUser, SMTPPassword, SMTPMailDomain, SMTPFrom                                                                                         string
	SMTPPort                                                                                                                                                        int
	EMQXBroker, EMQXClientID, EMQXUsername, EMQXToken                                                                                                               string
	DeviceCACertFile, DeviceCAKeyFile, DeviceMQTTTLSURI, DeviceMQTTWSSURI                                                                                           string
	LocalTokenIssuer, LocalTokenSigningKID, LocalTokenPublicKeyDERBase64                                                                                            string
	SPADeploymentDescriptorURL, SPADeploymentSigningKID, SPADeploymentPublicKeyDERBase64                                                                            string
	IntegrationSecretPepper, IntegrationEncryptionKeyBase64                                                                                                         string
	EMQXSubTopics                                                                                                                                                   []string
	AllowStubAdapters, SimulatorCardTerminalAvailable                                                                                                               bool
}

func Load() Config {
	port, _ := strconv.Atoi(getenv("SMTP_PORT", "587"))
	return Config{
		HTTPAddr: getenv("HTTP_ADDR", ":8080"), AppEnv: getenv("APP_ENV", "dev"), PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://localhost:8080/public/v1"), APIVersion: getenv("API_VERSION", "2026-08-07"),
		DatabaseURL: os.Getenv("DATABASE_URL"), RLSDatabaseURL: os.Getenv("RLS_DATABASE_URL"), CORSAllowedOrigins: getenv("CORS_ALLOWED_ORIGINS", "*"), RabbitMQURL: os.Getenv("RABBITMQ_URL"), SMTPHost: os.Getenv("SMTP_HOST"), SMTPUser: os.Getenv("SMTP_USER"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPMailDomain: os.Getenv("SMTP_MAILDOMAIN"), SMTPFrom: os.Getenv("SMTP_FROM"), SMTPPort: port,
		AuthHMACKey: os.Getenv("AUTH_HMAC_KEY"), OIDCIssuer: os.Getenv("OIDC_ISSUER"), OIDCAudience: os.Getenv("OIDC_AUDIENCE"), OIDCJWKSURL: os.Getenv("OIDC_JWKS_URL"), BLESigningKey: getenv("BLE_SIGNING_KEY", "dev-only-ble-signing-key-32-bytes"),
		EMQXBroker: os.Getenv("EMQX_BROKER"), EMQXClientID: getenv("EMQX_CLIENT_ID", "beefiscal-backend"), EMQXUsername: os.Getenv("EMQX_USERNAME"), EMQXToken: os.Getenv("EMQX_TOKEN"), EMQXSubTopics: splitCSV(os.Getenv("EMQX_SUB_TOPICS")),
		DeviceCACertFile: os.Getenv("DEVICE_CA_CERT_FILE"), DeviceCAKeyFile: os.Getenv("DEVICE_CA_KEY_FILE"), DeviceMQTTTLSURI: os.Getenv("DEVICE_MQTT_TLS_URI"), DeviceMQTTWSSURI: os.Getenv("DEVICE_MQTT_WSS_URI"),
		LocalTokenIssuer: os.Getenv("LOCAL_FISCAL_TOKEN_ISSUER"), LocalTokenSigningKID: os.Getenv("LOCAL_FISCAL_TOKEN_SIGNING_KID"), LocalTokenPublicKeyDERBase64: os.Getenv("LOCAL_FISCAL_TOKEN_PUBLIC_KEY_DER_BASE64"),
		SPADeploymentDescriptorURL: os.Getenv("SPA_DEPLOYMENT_DESCRIPTOR_URL"), SPADeploymentSigningKID: os.Getenv("SPA_DEPLOYMENT_SIGNING_KID"), SPADeploymentPublicKeyDERBase64: os.Getenv("SPA_DEPLOYMENT_PUBLIC_KEY_DER_BASE64"),
		IntegrationSecretPepper: getenv("INTEGRATION_SECRET_PEPPER", "dev-only-integration-pepper-32-bytes"), IntegrationEncryptionKeyBase64: getenv("INTEGRATION_ENCRYPTION_KEY_BASE64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="),
		AllowStubAdapters: strings.EqualFold(getenv("ALLOW_STUB_ADAPTERS", "true"), "true"), SimulatorCardTerminalAvailable: strings.EqualFold(getenv("SIMULATOR_CARD_TERMINAL_AVAILABLE", "false"), "true"),
	}
}
func (c Config) Validate() error {
	if c.AppEnv == "prod" && c.AllowStubAdapters {
		return errors.New("ALLOW_STUB_ADAPTERS must be false in PROD")
	}
	if c.AppEnv == "prod" && c.SimulatorCardTerminalAvailable {
		return errors.New("SIMULATOR_CARD_TERMINAL_AVAILABLE must be false in PROD")
	}
	if c.AppEnv == "prod" && !httpsURL(c.PublicBaseURL) {
		return errors.New("PUBLIC_BASE_URL must use HTTPS in PROD")
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
	if c.AppEnv == "prod" && (c.OIDCIssuer == "" || c.OIDCAudience == "" || c.OIDCJWKSURL == "") {
		return errors.New("OIDC_ISSUER, OIDC_AUDIENCE and OIDC_JWKS_URL required in PROD")
	}
	if c.AppEnv == "prod" && (!httpsURL(c.OIDCIssuer) || !httpsURL(c.OIDCJWKSURL)) {
		return errors.New("OIDC issuer and JWKS URL must use HTTPS in PROD")
	}
	if c.AppEnv == "prod" && (c.RabbitMQURL == "" || c.SMTPHost == "" || c.SMTPUser == "" || c.SMTPPassword == "" || c.SMTPFrom == "" || c.SMTPPort < 1) {
		return errors.New("RabbitMQ and SMTP configuration required in PROD")
	}
	if c.AppEnv == "prod" && (len(c.IntegrationSecretPepper) < 32 || strings.Contains(c.IntegrationSecretPepper, "dev-") || c.IntegrationEncryptionKeyBase64 == "") {
		return errors.New("strong integration pepper and encryption key required in PROD")
	}
	if c.AppEnv == "prod" && (len(c.AuthHMACKey) < 32 || strings.Contains(c.AuthHMACKey, "dev-")) {
		return errors.New("strong AUTH_HMAC_KEY required in PROD for BeeMiniPOS tokens")
	}
	if c.AppEnv == "prod" && (len(c.BLESigningKey) < 32 || strings.Contains(c.BLESigningKey, "dev-")) {
		return errors.New("strong BLE_SIGNING_KEY required in PROD")
	}
	if c.AppEnv == "prod" && (c.DeviceCACertFile == "" || c.DeviceCAKeyFile == "" || !strings.HasPrefix(c.DeviceMQTTTLSURI, "ssl://")) {
		return errors.New("DEVICE_CA_CERT_FILE, DEVICE_CA_KEY_FILE and ssl:// DEVICE_MQTT_TLS_URI required in PROD")
	}
	if c.AppEnv == "prod" && (c.LocalTokenIssuer == "" || c.LocalTokenSigningKID == "" || c.LocalTokenPublicKeyDERBase64 == "") {
		return errors.New("local fiscal token verifier trust required in PROD")
	}
	if c.AppEnv == "prod" && (!httpsURL(c.SPADeploymentDescriptorURL) || c.SPADeploymentSigningKID == "" || c.SPADeploymentPublicKeyDERBase64 == "") {
		return errors.New("signed HTTPS SPA deployment trust required in PROD")
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
