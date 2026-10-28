package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"fiscalisation/fiscal-backend/internal/auth"
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
)

// Handler is intentionally thin: authentication, versioning and generated
// request/response validation live at HTTP boundaries while fiscal decisions
// are delegated to domain.Service.
type Handler struct {
	svc *domain.Service
	cfg config.Config
}

func hasAnyRole(r *http.Request, allowed ...string) bool {
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		return false
	}
	for _, actual := range claims.Roles {
		for _, expected := range allowed {
			if strings.EqualFold(actual, expected) {
				return true
			}
		}
	}
	return false
}

var apiUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type webhookCreatedResponse struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Secret    string    `json:"secret"`
}

func typedWebhookCreated(v map[string]any) (webhookCreatedResponse, error) {
	var out webhookCreatedResponse
	allowed := map[string]bool{"id": true, "version": true, "url": true, "events": true, "status": true, "created_at": true, "updated_at": true, "secret": true}
	if len(v) != len(allowed) {
		return out, errors.New("webhook response property count")
	}
	for key := range v {
		if !allowed[key] {
			return out, errors.New("webhook response contains undocumented property")
		}
	}
	var ok bool
	out.ID, ok = v["id"].(string)
	if !ok || !apiUUIDPattern.MatchString(out.ID) {
		return out, errors.New("invalid webhook response id")
	}
	switch version := v["version"].(type) {
	case int64:
		out.Version = version
	case int:
		out.Version = int64(version)
	default:
		return out, errors.New("invalid webhook response version")
	}
	if out.Version < 1 {
		return out, errors.New("invalid webhook response version")
	}
	out.URL, ok = v["url"].(string)
	parsedURL, parseErr := url.Parse(out.URL)
	if !ok || parseErr != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return out, errors.New("invalid webhook response url")
	}
	rawEvents, ok := v["events"].([]any)
	if !ok || len(rawEvents) == 0 {
		return out, errors.New("invalid webhook response events")
	}
	seen := map[string]bool{}
	for _, raw := range rawEvents {
		event, valid := raw.(string)
		if !valid || event == "" || seen[event] {
			return out, errors.New("invalid webhook response event")
		}
		seen[event] = true
		out.Events = append(out.Events, event)
	}
	out.Status, ok = v["status"].(string)
	if !ok || out.Status != "ACTIVE" {
		return out, errors.New("invalid webhook response status")
	}
	out.CreatedAt, ok = v["created_at"].(time.Time)
	if !ok || out.CreatedAt.IsZero() {
		return out, errors.New("invalid webhook response created_at")
	}
	out.UpdatedAt, ok = v["updated_at"].(time.Time)
	if !ok || out.UpdatedAt.IsZero() {
		return out, errors.New("invalid webhook response updated_at")
	}
	out.Secret, ok = v["secret"].(string)
	secret, decodeErr := base64.RawURLEncoding.DecodeString(out.Secret)
	if !ok || decodeErr != nil || len(out.Secret) != 43 || len(secret) != 32 {
		return out, errors.New("invalid webhook response secret")
	}
	return out, nil
}

func NewHandler(s *domain.Service, c config.Config) http.Handler {
	return newHandler(s, c, nil, nil)
}

func NewHandlerWithIntegration(s *domain.Service, c config.Config, integrationHandler http.Handler) http.Handler {
	return newHandler(s, c, integrationHandler, nil)
}

func NewHandlerWithIntegrationRevocation(s *domain.Service, c config.Config, integrationHandler http.Handler, revoked func(context.Context, string) bool) http.Handler {
	return newHandler(s, c, integrationHandler, revoked)
}

func newHandler(s *domain.Service, c config.Config, integrationHandler http.Handler, revoked func(context.Context, string) bool) http.Handler {
	h := &Handler{s, c}
	m := http.NewServeMux()
	m.HandleFunc("/connectivity/ping", h.ping)
	m.HandleFunc("/public/v1/connectivity/ping", h.ping)
	m.HandleFunc("/livez", h.live)
	m.HandleFunc("/readyz", h.ready)
	m.HandleFunc("/metrics", metricsHandler)
	m.HandleFunc("/healthz", h.ready)
	m.HandleFunc("/public/v1/version", h.version)
	m.HandleFunc("/admin/v1/installed-versions", h.installedVersions)
	m.HandleFunc("/public/v1/sales", h.sales)
	m.HandleFunc("/public/v1/sales:open-with-line", h.openSaleWithLine)
	m.HandleFunc("/public/v1/sales/", h.sale)
	m.HandleFunc("/public/v1/operations/", h.operation)
	m.HandleFunc("/public/v1/operations", h.operations)
	m.HandleFunc("/public/v1/devices/", h.device)
	m.HandleFunc("/public/v1/devices", h.resourceCollection("device"))
	m.HandleFunc("/public/v1/organizations", h.organization)
	m.HandleFunc("/public/v1/locations", h.resourceCollection("location"))
	m.HandleFunc("/public/v1/locations/", h.resourceItem("location", "/public/v1/locations/"))
	m.HandleFunc("/public/v1/registers", h.resourceCollection("register"))
	m.HandleFunc("/public/v1/shifts", h.shifts)
	m.HandleFunc("/public/v1/shifts/", h.shift)
	m.HandleFunc("/public/v1/registers/", h.register)
	m.HandleFunc("/public/v1/workstations/", h.workstation)
	m.HandleFunc("/public/v1/connectivity-probes/", h.connectivityProbe)
	m.HandleFunc("/public/v1/operators", h.resourceCollection("operator"))
	m.HandleFunc("/public/v1/operators/", h.resourceItem("operator", "/public/v1/operators/"))
	m.HandleFunc("/public/v1/reports", h.reports)
	m.HandleFunc("/public/v1/reports/", h.report)
	m.HandleFunc("/public/v1/audit-events", h.auditEvents)
	m.HandleFunc("/public/v1/exports", h.exports)
	m.HandleFunc("/public/v1/exports/periodized", h.periodizedExports)
	m.HandleFunc("/public/v1/exports/", h.export)
	m.HandleFunc("/public/v1/tax-groups", h.taxGroups)
	m.HandleFunc("/public/v1/country-policy", h.countryPolicy)
	m.HandleFunc("/public/v1/webhook-endpoints", h.webhookEndpoints)
	m.HandleFunc("/public/v1/webhook-endpoints/", h.webhookEndpoint)
	m.HandleFunc("/public/v1/ble-sessions/", h.bleSession)
	m.HandleFunc("/public/v1/edge-sync/batches", h.sync)
	m.HandleFunc("/public/v1/payment-reconciliations:authorize", h.paymentReconciliations)
	m.HandleFunc("/public/v1/payment-reconciliations:bind-administrator", h.reconciliationAdministrator)
	m.HandleFunc("/public/v1/execution-authorizations", h.executionAuthorizations)
	m.HandleFunc("/public/v1/operators:bind-identity", h.operatorIdentityBinding)
	m.HandleFunc("/public/v1/device-activation-requests/", h.deviceActivationConfirmation)
	m.HandleFunc("/public/v1/device-activation-requests:lookup", h.deviceActivationLookup)
	m.HandleFunc("/public/v1/", func(w http.ResponseWriter, _ *http.Request) {
		problem(w, http.StatusNotFound, "NOT_FOUND")
	})
	m.HandleFunc("/device-bootstrap/v1/challenges", h.deviceBootstrapChallenge)
	m.HandleFunc("/device-bootstrap/v1/activation-requests", h.deviceBootstrapCreate)
	m.HandleFunc("/device-bootstrap/v1/activation-requests/", h.deviceBootstrapCredential)
	m.HandleFunc("/platform/v1/manufacturing/devices:register", h.manufacturingDeviceRegister)
	m.HandleFunc("/platform/v1/devices", h.platformDevices)
	m.HandleFunc("/platform/v1/devices/", h.platformDevice)
	if integrationHandler != nil {
		m.Handle("/platform/v1/tenants", integrationHandler)
		m.Handle("/platform/v1/external-systems", integrationHandler)
		m.Handle("/platform/v1/external-systems/", integrationHandler)
		m.Handle("/platform/v1/webhook-deliveries/", integrationHandler)
		m.Handle("/platform/v1/enrollment-conflicts", integrationHandler)
		m.Handle("/platform/v1/enrollment-conflicts/", integrationHandler)
		m.Handle("/platform/v1/integration-metrics", integrationHandler)
	}
	oidc := auth.NewOIDCVerifier(c.OIDCIssuer, c.OIDCAudience, c.OIDCJWKSURL)
	business := auth.MiddlewareWithOIDCAndRevocation(c.AuthHMACKey, oidc, revoked, rateLimit(600, time.Minute, recoverer(version(c.APIVersion, enforceSuccessResponses(repositoryAvailability(s, fiscalIdempotency(s, m)))))))
	pingLimiter := &requestLimiter{limit: 120, window: time.Minute, now: time.Now, entries: make(map[string]rateWindow)}
	pingRoute := pingLimiter.middlewareAll(http.HandlerFunc(h.ping))
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if integrationHandler != nil && (strings.HasPrefix(r.URL.Path, "/integration/v1/") || strings.HasPrefix(r.URL.Path, "/public/v1/app-auth/")) {
			integrationHandler.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/connectivity/ping" || r.URL.Path == "/public/v1/connectivity/ping" {
			pingRoute.ServeHTTP(w, r)
			return
		}
		business.ServeHTTP(w, r)
	})
	return cors(c.CORSAllowedOrigins, root)
}

var _ = json.Marshal
var _ = errors.Is
