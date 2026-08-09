package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fiscalisation/beeminipos-backend/internal/auth"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
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
	m.HandleFunc("/public/v1/minipos/shifts", h.shifts)
	m.HandleFunc("/public/v1/minipos/shifts/", h.shift)
	m.HandleFunc("/public/v1/minipos/orders", h.orders)
	m.HandleFunc("/public/v1/minipos/orders/", h.order)
	m.HandleFunc("/public/v1/minipos/reports/sales", h.salesReport)
	m.HandleFunc("/public/v1/fiscal-webhooks", h.webhook)
	m.HandleFunc("/public/v1/minipos/configuration", h.configuration)
	oidc := auth.NewOIDCVerifier(c.OIDCIssuer, c.OIDCAudience, c.OIDCJWKSURL)
	return cors(c.CORSAllowedOrigins, auth.MiddlewareWithOIDC(c.AuthHMACKey, oidc, rateLimit(600, time.Minute, apiVersion(c.APIVersion, idempotency(s, m)))))
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
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			problem(w, 400, "invalid body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:])
		tenant := tenantID(r)
		replayKey := tenant + "\n" + r.Method + "\n" + r.URL.Path + "\n" + r.Header.Get("Idempotency-Key")
		if old, ok := s.APIReplay(replayKey); ok {
			if old.Hash != hash {
				problem(w, 409, "idempotency payload mismatch")
				return
			}
			w.Header().Set("Idempotency-Replayed", "true")
			if old.ContentType != "" {
				w.Header().Set("Content-Type", old.ContentType)
			}
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
			if err = s.PutAPIReplay(replayKey, domain.APIReplay{Hash: hash, Status: cw.status, Body: append([]byte(nil), cw.body.Bytes()...), ContentType: cw.header.Get("Content-Type")}); err != nil {
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
			State       string `json:"state"`
			OperationID string `json:"operation_id"`
			ExternalID  string `json:"external_id"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &v) != nil || v.EventID == "" || v.EventType == "" || v.APIVersion != h.c.APIVersion || v.TenantID == "" || v.ResourceID == "" || v.ResourceVersion < 1 {
		problem(w, 400, "invalid json")
		return
	}
	if e = h.s.ProcessFiscalWebhookLinkedForTenant(v.TenantID, v.EventID, b, v.ResourceID, v.Data.ExternalID, v.Data.State, v.Data.OperationID, v.ResourceVersion); e != nil {
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
		write(w, 200, map[string]any{"items": h.s.ProductsFor(tenantID(r)), "page": map[string]any{"has_more": false}})
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
		write(w, 200, map[string]any{"items": h.s.EmployeesFor(tenantID(r)), "page": map[string]any{"has_more": false}})
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
	id := lastID(r.URL.Path, "employees")
	if id == "" {
		problem(w, 404, "not found")
		return
	}
	if !h.authorizeEmployee(w, r, id) {
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
func (h *handler) shifts(w http.ResponseWriter, r *http.Request) {
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
	if len(p) != 2 || p[1] != "close" || r.Method != "POST" {
		problem(w, 404, "not found")
		return
	}
	if !h.authorizeShift(w, r, p[0]) {
		return
	}
	v, e := h.s.CloseShiftForTenant(p[0], tenantID(r))
	if e != nil {
		problem(w, 409, e.Error())
		return
	}
	write(w, 200, v)
}
func (h *handler) orders(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		write(w, 200, map[string]any{"items": h.s.OrdersFor(tenantID(r)), "page": map[string]any{"has_more": false}})
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
	v.TenantID = tenantID(r)
	x, e := h.s.CreateOrder(v)
	if e != nil {
		problem(w, 409, e.Error())
		return
	}
	write(w, 201, x)
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
	if p[1] == "checkout" {
		var v map[string]any
		if !decode(w, r, &v) {
			return
		}
		x, e := h.s.CheckoutForTenant(p[0], r.Header.Get("Idempotency-Key"), v, tenantID(r))
		if e != nil {
			problem(w, 502, e.Error())
			return
		}
		write(w, 202, x)
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
	write(w, 200, h.s.SalesReportFor(tenantID(r)))
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
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, s int, d string) {
	write(w, s, map[string]any{"status": s, "code": "MINIPOS_ERROR", "detail": d})
}
func tenantID(r *http.Request) string {
	if c, ok := auth.ClaimsFrom(r.Context()); ok {
		return c.TenantID
	}
	return ""
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
	_, e := h.s.OrderForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "not found")
		return false
	}
	return true
}
func (h *handler) authorizeShift(w http.ResponseWriter, r *http.Request, id string) bool {
	_, e := h.s.ShiftForTenant(id, tenantID(r))
	if e != nil {
		problem(w, 404, "not found")
		return false
	}
	return true
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
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match, X-Api-Version")
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
