package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fiscalisation/fiscal-backend/internal/auth"
)

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if n := len(r.Header.Get("Idempotency-Key")); n < 16 || n > 255 {
		problem(w, 400, "IDEMPOTENCY_KEY_REQUIRED")
		return nil, false
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		problem(w, 400, "INVALID_BODY")
		return nil, false
	}
	return b, true
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func tenantID(r *http.Request) string {
	if c, ok := auth.ClaimsFrom(r.Context()); ok {
		return c.TenantID
	}
	return ""
}

func actorSubject(r *http.Request) string {
	if c, ok := auth.ClaimsFrom(r.Context()); ok {
		return c.Subject
	}
	return ""
}

func replayKey(r *http.Request) string {
	return tenantID(r) + " " + r.Method + " " + r.URL.Path + " " + r.Header.Get("Idempotency-Key")
}

func paginate[T any](r *http.Request, all []T) (map[string]any, error) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			return nil, errors.New("invalid limit")
		}
		limit = parsed
	}
	start := 0
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return nil, errors.New("invalid cursor")
		}
		if _, err = fmt.Sscanf(string(decoded), "offset:%d", &start); err != nil || start < 0 || start > len(all) {
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

func optionalTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func (h *Handler) authorizeSale(w http.ResponseWriter, r *http.Request, id string) bool {
	tenant := tenantID(r)
	if tenant == "" {
		problem(w, 401, "UNAUTHORIZED")
		return false
	}
	_, e := h.svc.GetSaleForTenant(id, tenant)
	if e != nil {
		problem(w, 404, "SALE_NOT_FOUND")
		return false
	}
	return true
}

func problem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	slug := strings.ReplaceAll(strings.ToLower(code), "_", "-")
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "urn:beefiscal:error:" + slug, "title": code, "status": status, "detail": code, "code": code, "retryable": false, "trace_id": time.Now().UTC().Format("20060102150405.000000000")})
}

func version(v string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/public/v1") && r.Header.Get("X-Api-Version") != v {
			problem(w, 400, "API_VERSION_REQUIRED")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				problem(w, 500, "INTERNAL")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func cors(allowed string, next http.Handler) http.Handler {
	wildcard := strings.TrimSpace(allowed) == "*"
	set := map[string]bool{}
	for _, v := range strings.Split(allowed, ",") {
		set[strings.TrimSpace(v)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if wildcard || (origin != "" && set[origin]) {
			w.Header().Set("Access-Control-Allow-Origin", map[bool]string{true: "*", false: origin}[wildcard])
			if !wildcard {
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match, X-Api-Version, X-Tenant-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			if !wildcard && (origin == "" || !set[origin]) {
				problem(w, 403, "ORIGIN_NOT_ALLOWED")
				return
			}
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SignWebhook(key string, b []byte) string {
	m := hmac.New(sha256.New, []byte(key))
	_, _ = io.Copy(m, bytes.NewReader(b))
	return hex.EncodeToString(m.Sum(nil))
}

func ifMatch(r *http.Request) (int64, bool) {
	v := strings.Trim(r.Header.Get("If-Match"), `"`)
	if v == "" {
		return 0, false
	}
	n, e := strconv.ParseInt(v, 10, 64)
	return n, e == nil
}

func decodeStrict(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		problem(w, 400, "INVALID_BODY")
		return false
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		problem(w, 422, "REQUEST_INVALID")
		return false
	}
	return true
}
