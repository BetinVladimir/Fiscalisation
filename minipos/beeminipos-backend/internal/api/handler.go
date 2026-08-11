package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type handler struct {
	s *domain.Service
	c config.Config
}

func New(s *domain.Service, c config.Config) http.Handler {
	h := &handler{s, c}
	m := http.NewServeMux()
	m.HandleFunc("/livez", h.health)
	m.HandleFunc("/readyz", h.health)
	m.HandleFunc("/healthz", h.health)
	m.HandleFunc("/api/v1/products", h.products)
	m.HandleFunc("/api/v1/products/", h.product)
	m.HandleFunc("/api/v1/employees", h.employees)
	m.HandleFunc("/api/v1/employees/", h.employee)
	m.HandleFunc("/api/v1/shifts", h.shifts)
	m.HandleFunc("/api/v1/shifts/", h.shift)
	m.HandleFunc("/api/v1/orders", h.orders)
	m.HandleFunc("/api/v1/orders/", h.order)
	m.HandleFunc("/api/v1/fiscal-webhooks", h.webhook)
	m.HandleFunc("/api/v1/configuration", h.configuration)
	m.HandleFunc("/public/v1/minipos/products", h.products)
	m.HandleFunc("/public/v1/minipos/products/", h.product)
	m.HandleFunc("/public/v1/minipos/employees", h.employees)
	m.HandleFunc("/public/v1/minipos/employees/", h.employee)
	m.HandleFunc("/public/v1/minipos/operator-session", h.operatorSession)
	m.HandleFunc("/public/v1/minipos/shifts", h.shifts)
	m.HandleFunc("/public/v1/minipos/shifts/", h.shift)
	m.HandleFunc("/public/v1/minipos/orders", h.orders)
	m.HandleFunc("/public/v1/minipos/orders/", h.order)
	m.HandleFunc("/public/v1/minipos/reports/sales", h.salesReport)
	m.HandleFunc("/public/v1/fiscal-webhooks", h.webhook)
	m.HandleFunc("/public/v1/minipos/configuration", h.configuration)
	oidc := auth.NewOIDCVerifier(c.OIDCIssuer, c.OIDCAudience, c.OIDCJWKSURL)
	return cors(c.CORSAllowedOrigins, auth.MiddlewareWithOIDCAndRevocation(c.AuthHMACKey, oidc, func(claims auth.Claims) bool {
		return s.OperatorSessionRevoked(claims.TenantID, claims.TokenHash)
	}, rateLimit(600, time.Minute, apiVersion(c.APIVersion, enforceSuccessResponses(idempotency(s, m))))))
}

func (h *handler) configuration(w http.ResponseWriter, r *http.Request) {
	tenant := tenantID(r)
	if r.Method == http.MethodGet {
		v, err := h.s.ConfigurationFor(tenant)
		if err != nil {
			problem(w, http.StatusNotFound, "configuration not found")
			return
		}
		write(w, http.StatusOK, v)
		return
	}
	if r.Method != http.MethodPatch {
		problem(w, http.StatusMethodNotAllowed, "method")
		return
	}
	var v domain.Configuration
	if !decode(w, r, &v) {
		return
	}
	expected := int64(0)
	if raw := strings.Trim(r.Header.Get("If-Match"), `"`); raw != "" {
		var err error
		expected, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || expected < 1 {
			problem(w, http.StatusBadRequest, "invalid If-Match")
			return
		}
	}
	result, err := h.s.SaveConfiguration(tenant, expected, v)
	if err != nil {
		problem(w, http.StatusConflict, err.Error())
		return
	}
	write(w, http.StatusOK, result)
}

type capturedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *capturedResponse) Header() http.Header { return w.header }
func (w *capturedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *capturedResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(b)
}
func idempotency(s *domain.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/public/v1") || r.URL.Path == "/public/v1/fiscal-webhooks" || (r.Method != "POST" && r.Method != "PATCH" && r.Method != "DELETE") {
			next.ServeHTTP(w, r)
			return
		}
		if n := len(r.Header.Get("Idempotency-Key")); n < 16 || n > 255 {
			problem(w, 400, "idempotency key must be 16..255 characters")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			problem(w, 400, "invalid body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		hash := idempotencyFingerprint(r, body)
		tenant := tenantID(r)
		replayKey := tenant + "\n" + r.Method + "\n" + r.URL.Path + "\n" + r.Header.Get("Idempotency-Key")
		old, owner, claimErr := s.ClaimAPIReplay(replayKey, hash)
		if claimErr != nil {
			if errors.Is(claimErr, domain.ErrAPIReplayMismatch) {
				problem(w, 409, "idempotency payload mismatch")
			} else {
				problem(w, 503, "idempotency reservation failed")
			}
			return
		}
		if !owner {
			if old.Pending {
				applyReplayHeaders(w.Header(), old.Headers, old.ContentType)
				w.Header().Set("Retry-After", "1")
				if old.Status >= 500 && len(old.Body) > 0 {
					w.Header().Set("Idempotency-Replayed", "true")
					w.WriteHeader(old.Status)
					_, _ = w.Write(old.Body)
				} else {
					problem(w, 409, "idempotency request in progress")
				}
				return
			}
			w.Header().Set("Idempotency-Replayed", "true")
			applyReplayHeaders(w.Header(), old.Headers, old.ContentType)
			w.WriteHeader(old.Status)
			_, _ = w.Write(old.Body)
			return
		}
		cw := &capturedResponse{header: make(http.Header)}
		next.ServeHTTP(cw, r)
		if cw.status == 0 {
			cw.status = 200
		}
		if cw.status < 500 {
			if err = s.PutAPIReplay(replayKey, domain.APIReplay{Hash: hash, Status: cw.status, Body: append([]byte(nil), cw.body.Bytes()...), ContentType: cw.header.Get("Content-Type"), Headers: replayHeaders(cw.header)}); err != nil {
				problem(w, 500, "idempotency persistence failed")
				return
			}
		} else {
			// Keep the durable PENDING claim after a server-side ambiguity. A
			// retry must reconcile instead of re-executing a possible side effect.
			cw.header.Set("Retry-After", "1")
			if err = s.PutAPIAmbiguousReplay(replayKey, domain.APIReplay{Hash: hash, Status: cw.status, Body: append([]byte(nil), cw.body.Bytes()...), ContentType: cw.header.Get("Content-Type"), Headers: replayHeaders(cw.header), Pending: true}); err != nil {
				problem(w, 500, "idempotency persistence failed")
				return
			}
		}
		for k, values := range cw.header {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(cw.status)
		_, _ = w.Write(cw.body.Bytes())
	})
}

var replayableHeader = map[string]bool{"Content-Type": true, "Etag": true, "Location": true, "Retry-After": true, "Cache-Control": true}

func replayHeaders(source http.Header) map[string][]string {
	result := map[string][]string{}
	for key, values := range source {
		canonical := http.CanonicalHeaderKey(key)
		if replayableHeader[canonical] {
			result[canonical] = append([]string(nil), values...)
		}
	}
	return result
}

func applyReplayHeaders(target http.Header, headers map[string][]string, legacyContentType string) {
	for key, values := range headers {
		canonical := http.CanonicalHeaderKey(key)
		if !replayableHeader[canonical] {
			continue
		}
		for _, value := range values {
			target.Add(canonical, value)
		}
	}
	if target.Get("Content-Type") == "" && legacyContentType != "" {
		target.Set("Content-Type", legacyContentType)
	}
}

func idempotencyFingerprint(r *http.Request, body []byte) string {
	h := sha256.New()
	issuer, subject := "", ""
	if claims, ok := auth.ClaimsFrom(r.Context()); ok {
		issuer, subject = claims.Issuer, claims.Subject
	}
	for _, value := range []string{r.URL.Query().Encode(), r.Header.Get("If-Match"), r.Header.Get("X-App-Instance-Id"), issuer, subject} {
		_, _ = fmt.Fprintf(h, "%d:", len(value))
		_, _ = h.Write([]byte(value))
	}
	_, _ = fmt.Fprintf(h, "%d:", len(body))
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
func (h *handler) webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "method")
		return
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		problem(w, 400, "invalid body")
		return
	}
	if !validWebhookSignature(r.Header.Get("BeeFiscal-Signature"), b, []byte(h.c.WebhookVerificationKey), time.Now().UTC()) {
		problem(w, 401, "invalid signature")
		return
	}
	var v struct {
		EventID         string `json:"event_id"`
		EventType       string `json:"event_type"`
		APIVersion      string `json:"api_version"`
		TenantID        string `json:"tenant_id"`
		ResourceID      string `json:"resource_id"`
		ResourceVersion int64  `json:"resource_version"`
		Data            struct {
			State           string `json:"state"`
			OperationID     string `json:"operation_id"`
			OperationType   string `json:"operation_type"`
			ExternalID      string `json:"external_id"`
			FiscalReference string `json:"fiscal_reference"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &v) != nil || v.EventID == "" || v.EventType == "" || v.APIVersion != h.c.APIVersion || v.TenantID == "" || v.ResourceID == "" || v.ResourceVersion < 1 {
		problem(w, 400, "invalid json")
		return
	}
	if r.Header.Get("BeeFiscal-Event-Id") != v.EventID {
		problem(w, http.StatusBadRequest, "WEBHOOK_EVENT_ID_MISMATCH")
		return
	}
	if strings.EqualFold(h.c.AppEnv, "prod") {
		if _, err := h.s.ConfigurationFor(v.TenantID); err != nil {
			problem(w, http.StatusForbidden, "WEBHOOK_TENANT_NOT_CONFIGURED")
			return
		}
	}
	state := v.Data.State
	if v.Data.OperationType == "REVERSAL" && state == "FISCALIZED" {
		state = "REVERSED"
	}
	if e = h.s.ProcessFiscalWebhookLinkedForTenantWithReference(v.TenantID, v.EventID, b, v.ResourceID, v.Data.ExternalID, state, v.Data.OperationID, v.Data.FiscalReference, v.ResourceVersion); e != nil {
		problem(w, 409, e.Error())
		return
	}
	w.WriteHeader(204)
}

func validWebhookSignature(raw string, body, key []byte, now time.Time) bool {
	parts := strings.Split(raw, ",")
	if len(parts) != 3 {
		return false
	}
	fields := map[string]string{}
	for _, part := range parts {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 || fields[pair[0]] != "" {
			return false
		}
		fields[pair[0]] = pair[1]
	}
	seconds, err := strconv.ParseInt(fields["t"], 10, 64)
	if err != nil || fields["kid"] == "" || fields["v1"] == "" {
		return false
	}
	delta := now.Unix() - seconds
	if delta < -300 || delta > 300 {
		return false
	}
	got, err := hex.DecodeString(fields["v1"])
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(fields["t"] + "."))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}
func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	write(w, 200, map[string]string{"status": "ok"})
}
func (h *handler) products(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, err := paginate(r, h.s.ProductsFor(tenantID(r)))
		if err != nil {
			problem(w, http.StatusBadRequest, "INVALID_PAGINATION")
			return
		}
		write(w, 200, page)
		return
	}
	if r.Method == "POST" {
		var v domain.Product
		if !decode(w, r, &v) {
			return
		}
		v.TenantID = tenantID(r)
		x, e := h.s.CreateProduct(v)
		if e != nil {
			problem(w, 422, e.Error())
			return
		}
		write(w, 201, x)
		return
	}
	problem(w, 405, "method")
}
func (h *handler) product(w http.ResponseWriter, r *http.Request) {
	id := lastID(r.URL.Path, "products")
	if id == "" {
		problem(w, 404, "not found")
		return
	}
	if !h.authorizeProduct(w, r, id) {
		return
	}
	if r.Method == "GET" {
		v, e := h.s.ProductForTenant(id, tenantID(r))
		if e != nil {
			problem(w, 404, e.Error())
			return
		}
		write(w, 200, v)
		return
	}
	if r.Method == "PATCH" {
		expected, e := strconv.ParseInt(strings.Trim(r.Header.Get("If-Match"), `"`), 10, 64)
		if e != nil {
			problem(w, 400, "If-Match required")
			return
		}
		var v domain.Product
		if !decode(w, r, &v) {
			return
		}
		v.TenantID = tenantID(r)
		x, e := h.s.UpdateProductForTenant(id, expected, v, tenantID(r))
		if e != nil {
			problem(w, 409, e.Error())
			return
		}
		write(w, 200, x)
		return
	}
	problem(w, 405, "method")
}
func (h *handler) employees(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, err := paginate(r, h.s.EmployeesFor(tenantID(r)))
		if err != nil {
			problem(w, http.StatusBadRequest, "INVALID_PAGINATION")
			return
		}
		write(w, 200, page)
		return
	}
	if r.Method == "POST" {
		var v domain.Employee
		if !decode(w, r, &v) {
			return
		}
		v.TenantID = tenantID(r)
		x, e := h.s.CreateEmployee(v)
		if e != nil {
			problem(w, 422, e.Error())
			return
		}
		write(w, 201, x)
		return
	}
	problem(w, 405, "method")
}
func (h *handler) employee(w http.ResponseWriter, r *http.Request) {
	tail := lastID(r.URL.Path, "employees")
	parts := strings.Split(tail, "/")
	id := parts[0]
	if id == "" {
		problem(w, 404, "not found")
		return
	}
	if !h.authorizeEmployee(w, r, id) {
		return
	}
	if len(parts) == 2 && parts[1] == "identity-binding" {
		claims, authenticated := auth.ClaimsFrom(r.Context())
		if !authenticated || !claimHasRole(claims, "ADMIN") {
			problem(w, http.StatusForbidden, "administrator required")
			return
		}
		if r.Method != http.MethodPost {
			problem(w, http.StatusMethodNotAllowed, "method")
			return
		}
		var input struct {
			Subject string `json:"subject"`
			Issuer  string `json:"issuer"`
		}
		if !decode(w, r, &input) {
			return
		}
		binding, err := h.s.BindEmployeeIdentity(claims.TenantID, id, input.Subject, input.Issuer)
		if err != nil {
			problem(w, http.StatusConflict, err.Error())
			return
		}
		write(w, http.StatusCreated, map[string]any{"employee_id": binding.EmployeeID, "issuer": binding.IdentityIssuer, "bound_at": binding.BoundAt})
		return
	}
	if len(parts) != 1 {
		problem(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method == "GET" {
		v, e := h.s.EmployeeForTenant(id, tenantID(r))
		if e != nil {
			problem(w, 404, e.Error())
			return
		}
		write(w, 200, v)
		return
	}
	if r.Method == "PATCH" {
		expected, e := strconv.ParseInt(strings.Trim(r.Header.Get("If-Match"), `"`), 10, 64)
		if e != nil {
			problem(w, 400, "If-Match required")
			return
		}
		var v domain.Employee
		if !decode(w, r, &v) {
			return
		}
		v.TenantID = tenantID(r)
		x, e := h.s.UpdateEmployeeForTenant(id, expected, v, tenantID(r))
		if e != nil {
			problem(w, 409, e.Error())
			return
		}
		write(w, 200, x)
		return
	}
	problem(w, 405, "method")
}
func (h *handler) operatorSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		problem(w, http.StatusMethodNotAllowed, "method")
		return
	}
	claims, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		problem(w, http.StatusUnauthorized, "authenticated operator required")
		return
	}
	appInstanceID := strings.TrimSpace(r.Header.Get("X-App-Instance-Id"))
	if appInstanceID == "" {
		problem(w, http.StatusBadRequest, "X-App-Instance-Id required")
		return
	}
	if r.Method == http.MethodPost {
		if _, err := h.s.RevokeOperatorSession(claims.TenantID, claims.TokenHash); err != nil {
			problem(w, http.StatusConflict, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	employee, err := h.s.EmployeeForIdentity(claims.TenantID, claims.Issuer, claims.Subject)
	if err != nil {
		problem(w, http.StatusForbidden, "operator identity is not bound")
		return
	}
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	if _, err = h.s.RegisterOperatorSession(claims.TenantID, employee.ID, appInstanceID, claims.TokenHash, expiresAt); err != nil {
		problem(w, http.StatusForbidden, err.Error())
		return
	}
	write(w, http.StatusOK, domain.OperatorSession{Authenticated: true, Employee: &employee, SessionID: claims.TokenHash, ExpiresAt: expiresAt})
}
func (h *handler) shifts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		employeeID := strings.TrimSpace(r.URL.Query().Get("employee_id"))
		state := strings.TrimSpace(r.URL.Query().Get("state"))
		if employeeID == "" || state != "" && state != "OPEN" && state != "CLOSING" && state != "BLOCKED_RECONCILIATION" && state != "CLOSED" {
			problem(w, http.StatusBadRequest, "invalid shift recovery filter")
			return
		}
		if !h.authorizeEmployeeActor(w, r, employeeID) {
			return
		}
		items := h.s.ShiftsFor(tenantID(r), employeeID, strings.TrimSpace(r.URL.Query().Get("register_id")), state)
		write(w, http.StatusOK, map[string]any{"items": items, "page": map[string]any{"next_cursor": nil, "has_more": false}})
		return
	}
	if r.Method != "POST" {
		problem(w, 405, "method")
		return
	}
	var v struct {
		RegisterID string `json:"register_id"`
		EmployeeID string `json:"employee_id"`
	}
	if !decode(w, r, &v) {
		return
	}
	if !h.authorizeEmployeeActor(w, r, v.EmployeeID) {
		return
	}
	x, e := h.s.OpenShiftForTenant(v.RegisterID, v.EmployeeID, tenantID(r))
	if e != nil {
		problem(w, 409, e.Error())
		return
	}
	write(w, 201, x)
}
func (h *handler) shift(w http.ResponseWriter, r *http.Request) {
	i := strings.Index(r.URL.Path, "/shifts/")
	if i < 0 {
		problem(w, 404, "not found")
		return
	}
	p := strings.Split(strings.Trim(r.URL.Path[i+len("/shifts/"):], "/"), "/")
	if len(p) != 2 || (p[1] != "close" && p[1] != "reconcile") || r.Method != "POST" {
		problem(w, 404, "not found")
		return
	}
	if !h.authorizeShift(w, r, p[0]) {
		return
	}
	var v domain.Shift
	var e error
	if p[1] == "reconcile" {
		v, e = h.s.ReconcileShiftCloseForTenant(p[0], tenantID(r))
	} else {
		v, e = h.s.CloseShiftForTenant(p[0], tenantID(r))
	}
	if e != nil {
		problem(w, 409, e.Error())
		return
	}
	write(w, 200, v)
}
func (h *handler) orders(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		items := h.s.OrdersFor(tenantID(r))
		if claims, authenticated := auth.ClaimsFrom(r.Context()); authenticated && claimHasRole(claims, "CASHIER") && !claimHasRole(claims, "SUPERVISOR") && !claimHasRole(claims, "ADMIN") {
			employee, err := h.s.EmployeeForIdentity(claims.TenantID, claims.Issuer, claims.Subject)
			if err != nil {
				problem(w, http.StatusForbidden, "operator identity is not bound")
				return
			}
			if !h.s.OperatorSessionActive(claims.TenantID, claims.TokenHash, employee.ID, strings.TrimSpace(r.Header.Get("X-App-Instance-Id")), time.Now().UTC()) {
				problem(w, http.StatusUnauthorized, "operator session is not active")
				return
			}
			items = h.s.OrdersForEmployee(claims.TenantID, employee.ID)
		}
		page, err := paginate(r, items)
		if err != nil {
			problem(w, http.StatusBadRequest, "INVALID_PAGINATION")
			return
		}
		write(w, 200, page)
		return
	}
	if r.Method != "POST" {
		problem(w, 405, "method")
		return
	}
	var v domain.Order
	if !decode(w, r, &v) {
		return
	}
	if !h.authorizeShiftActor(w, r, v.ShiftID) {
		return
	}
	v.TenantID = tenantID(r)
	x, e := h.s.CreateOrder(v)
	if e != nil {
		problem(w, 409, e.Error())
		return
	}
	write(w, 201, x)
}

func paginate[T any](r *http.Request, all []T) (map[string]any, error) {
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			return nil, errors.New("invalid limit")
		}
		limit = parsed
	}
	start := 0
	if cursor := strings.TrimSpace(r.URL.Query().Get("cursor")); cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, errors.New("invalid cursor")
		}
		if n, err := fmt.Sscanf(string(decoded), "offset:%d", &start); err != nil || n != 1 || start < 0 || start > len(all) {
			return nil, errors.New("invalid cursor")
		}
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	hasMore := end < len(all)
	var next any
	if hasMore {
		next = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("offset:%d", end)))
	}
	return map[string]any{"items": all[start:end], "page": map[string]any{"next_cursor": next, "has_more": hasMore}}, nil
}
func (h *handler) order(w http.ResponseWriter, r *http.Request) {
	i := strings.Index(r.URL.Path, "/orders/")
	if i < 0 {
		problem(w, 404, "not found")
		return
	}
	p := strings.Split(strings.Trim(r.URL.Path[i+len("/orders/"):], "/"), "/")
	if len(p) > 0 && !h.authorizeOrder(w, r, p[0]) {
		return
	}
	if len(p) == 1 && r.Method == "GET" {
		x, e := h.s.OrderForTenant(p[0], tenantID(r))
		if e != nil {
			problem(w, 404, e.Error())
			return
		}
		write(w, 200, x)
		return
	}
	if len(p) != 2 || r.Method != "POST" {
		problem(w, 404, "not found")
		return
	}
	if r.Header.Get("Idempotency-Key") == "" {
		problem(w, 400, "idempotency required")
		return
	}
	if p[1] == "lines" {
		var v domain.Line
		if !decode(w, r, &v) {
			return
		}
		if v.Discount != nil && v.Discount.Amount != "0.00" {
			if claims, authenticated := auth.ClaimsFrom(r.Context()); authenticated && !claimHasRole(claims, "SUPERVISOR") && !claimHasRole(claims, "ADMIN") {
				problem(w, http.StatusForbidden, "manager role required for discount")
				return
			}
		}
		expected := int64(0)
		if strings.HasPrefix(r.URL.Path, "/public/v1/") {
			var ok bool
			expected, ok = parseIfMatch(r)
			if !ok {
				problem(w, 428, "If-Match required")
				return
			}
		}
		x, e := h.s.AddLineExpectedForTenant(p[0], expected, v, tenantID(r))
		if e != nil {
			problem(w, 409, e.Error())
			return
		}
		write(w, 200, x)
		return
	}
	if p[1] == "checkout" || p[1] == "checkout-batch" {
		var v map[string]any
		if !decode(w, r, &v) {
			return
		}
		var x domain.Order
		var e error
		if p[1] == "checkout-batch" {
			rawPayments, ok := v["payments"].([]any)
			if !ok {
				problem(w, 422, "payments are required")
				return
			}
			payments := make([]map[string]any, 0, len(rawPayments))
			for _, raw := range rawPayments {
				payment, valid := raw.(map[string]any)
				if !valid {
					problem(w, 422, "invalid payment")
					return
				}
				payments = append(payments, payment)
			}
			metadata, _ := v["metadata"].(map[string]any)
			x, e = h.s.CheckoutBatchForTenant(p[0], r.Header.Get("Idempotency-Key"), payments, metadata, tenantID(r))
		} else {
			x, e = h.s.CheckoutForTenant(p[0], r.Header.Get("Idempotency-Key"), v, tenantID(r))
		}
		if e != nil {
			problem(w, 502, e.Error())
			return
		}
		state := "UNKNOWN"
		switch x.State {
		case "COMPLETED":
			state = "FISCALIZED"
		case "FAILED":
			state = "FAILED"
		}
		write(w, 202, map[string]any{
			"operation_id":     x.FiscalOperationID,
			"type":             "FISCAL_SALE",
			"state":            state,
			"fiscal_reference": x.ReceiptReference,
			"allowed_actions":  []string{},
			"simulated":        true,
			"created_at":       x.UpdatedAt,
			"updated_at":       x.UpdatedAt,
		})
		return
	}
	if p[1] == "reversals" {
		var v struct {
			ReasonCode string `json:"reason_code"`
		}
		if !decode(w, r, &v) {
			return
		}
		x, e := h.s.ReverseOrderForTenant(p[0], r.Header.Get("Idempotency-Key"), v.ReasonCode, tenantID(r))
		if e != nil {
			problem(w, 502, e.Error())
			return
		}
		write(w, 202, map[string]any{
			"operation_id":     x.OperationID,
			"type":             "REVERSAL",
			"state":            x.State,
			"fiscal_reference": x.FiscalReference,
			"allowed_actions":  []string{},
			"simulated":        x.Simulated,
			"created_at":       x.CreatedAt,
			"updated_at":       x.UpdatedAt,
		})
		return
	}
	problem(w, 404, "not found")
}
func parseIfMatch(r *http.Request) (int64, bool) {
	v := strings.Trim(r.Header.Get("If-Match"), `"`)
	n, e := strconv.ParseInt(v, 10, 64)
	return n, e == nil && n > 0
}
func (h *handler) salesReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "method")
		return
	}
	from, fromErr := time.Parse(time.RFC3339, strings.TrimSpace(r.URL.Query().Get("from")))
	to, toErr := time.Parse(time.RFC3339, strings.TrimSpace(r.URL.Query().Get("to")))
	if fromErr != nil || toErr != nil || !from.Before(to) {
		problem(w, http.StatusBadRequest, "INVALID_REPORT_PERIOD")
		return
	}
	write(w, 200, h.s.SalesReportForPeriod(tenantID(r), from, to))
}
func lastID(path, resource string) string {
	i := strings.Index(path, "/"+resource+"/")
	if i < 0 {
		return ""
	}
	return strings.Trim(path[i+len(resource)+2:], "/")
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); e != nil {
		problem(w, 400, "invalid json")
		return false
	}
	return true
}
func write(w http.ResponseWriter, s int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(publicResponse(v))
}

func publicResponse(v any) any {
	switch value := v.(type) {
	case domain.Order:
		return publicOrder(value)
	case map[string]any:
		items, ok := value["items"].([]domain.Order)
		if !ok {
			return value
		}
		copyValue := make(map[string]any, len(value))
		for key, item := range value {
			copyValue[key] = item
		}
		publicItems := make([]map[string]any, 0, len(items))
		for _, order := range items {
			publicItems = append(publicItems, publicOrder(order))
		}
		copyValue["items"] = publicItems
		return copyValue
	default:
		return v
	}
}

func publicOrder(order domain.Order) map[string]any {
	raw, _ := json.Marshal(order)
	result := map[string]any{}
	_ = json.Unmarshal(raw, &result)
	delete(result, "payments")
	return result
}
func problem(w http.ResponseWriter, s int, d string) {
	code := d
	if !strings.Contains(code, "_") || strings.Contains(code, " ") {
		code = "MINIPOS_ERROR"
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "urn:beeminipos:error:" + strings.ToLower(code), "title": code, "status": s, "code": code, "retryable": false, "detail": d, "trace_id": time.Now().UTC().Format("20060102150405.000000000")})
}
func tenantID(r *http.Request) string {
	if c, ok := auth.ClaimsFrom(r.Context()); ok {
		return c.TenantID
	}
	return ""
}
func claimHasRole(claims auth.Claims, role string) bool {
	for _, current := range claims.Roles {
		if strings.EqualFold(current, role) {
			return true
		}
	}
	return false
}
func (h *handler) authorizeEmployeeActor(w http.ResponseWriter, r *http.Request, employeeID string) bool {
	claims, authenticated := auth.ClaimsFrom(r.Context())
	if !authenticated {
		if !strings.EqualFold(h.c.AppEnv, "prod") {
			return true
		}
		problem(w, http.StatusUnauthorized, "authenticated operator required")
		return false
	}
	if !h.s.IdentityMatchesEmployee(claims.TenantID, claims.Issuer, claims.Subject, employeeID) {
		problem(w, http.StatusForbidden, "operator identity does not match employee")
		return false
	}
	if !h.s.OperatorSessionActive(claims.TenantID, claims.TokenHash, employeeID, strings.TrimSpace(r.Header.Get("X-App-Instance-Id")), time.Now().UTC()) {
		problem(w, http.StatusUnauthorized, "operator session is not active")
		return false
	}
	return true
}
func (h *handler) authorizeShiftActor(w http.ResponseWriter, r *http.Request, shiftID string) bool {
	shift, err := h.s.ShiftForTenant(shiftID, tenantID(r))
	if err != nil {
		problem(w, http.StatusNotFound, "not found")
		return false
	}
	return h.authorizeEmployeeActor(w, r, shift.EmployeeID)
}
func (h *handler) authorizeProduct(w http.ResponseWriter, r *http.Request, id string) bool {
	_, e := h.s.ProductForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "not found")
		return false
	}
	return true
}
func (h *handler) authorizeEmployee(w http.ResponseWriter, r *http.Request, id string) bool {
	_, e := h.s.EmployeeForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "not found")
		return false
	}
	return true
}
func (h *handler) authorizeOrder(w http.ResponseWriter, r *http.Request, id string) bool {
	order, e := h.s.OrderForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "not found")
		return false
	}
	return h.authorizeShiftActor(w, r, order.ShiftID)
}
func (h *handler) authorizeShift(w http.ResponseWriter, r *http.Request, id string) bool {
	_, e := h.s.ShiftForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "not found")
		return false
	}
	return h.authorizeShiftActor(w, r, id)
}
func apiVersion(v string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/public/v1") && r.Header.Get("X-Api-Version") != v {
			problem(w, 400, "api version required")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/public/v1") && r.URL.Path != "/public/v1/fiscal-webhooks" && (r.Method == "POST" || r.Method == "PATCH" || r.Method == "DELETE") {
			n := len(r.Header.Get("Idempotency-Key"))
			if n < 16 || n > 255 {
				problem(w, 400, "idempotency key must be 16..255 characters")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func cors(allowed string, next http.Handler) http.Handler {
	set := map[string]bool{}
	for _, v := range strings.Split(allowed, ",") {
		set[strings.TrimSpace(v)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && set[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match, X-Api-Version, X-App-Instance-Id")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			if origin == "" || !set[origin] {
				problem(w, 403, "origin not allowed")
				return
			}
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
