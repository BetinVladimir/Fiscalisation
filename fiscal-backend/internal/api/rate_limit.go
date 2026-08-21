package api

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"fiscalisation/fiscal-backend/internal/auth"
)

type rateWindow struct {
	started time.Time
	count   int
}

type requestLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	entries map[string]rateWindow
}

func rateLimit(limit int, window time.Duration, next http.Handler) http.Handler {
	l := &requestLimiter{limit: limit, window: window, now: time.Now, entries: make(map[string]rateWindow)}
	return l.middleware(next)
}

func (l *requestLimiter) middleware(next http.Handler) http.Handler {
	return l.middlewareWhen(next, func(r *http.Request) bool { return isRateLimitedPath(r.URL.Path) })
}

func (l *requestLimiter) middlewareAll(next http.Handler) http.Handler {
	return l.middlewareWhen(next, func(*http.Request) bool { return true })
}

func (l *requestLimiter) middlewareWhen(next http.Handler, applies func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !applies(r) {
			next.ServeHTTP(w, r)
			return
		}
		key := rateLimitKey(r)
		now := l.now()
		l.mu.Lock()
		entry := l.entries[key]
		if entry.started.IsZero() || now.Sub(entry.started) >= l.window {
			entry = rateWindow{started: now}
		}
		entry.count++
		l.entries[key] = entry
		allowed := entry.count <= l.limit
		if allowed && now.Sub(entry.started) >= l.window {
			delete(l.entries, key)
		}
		retry := int(l.window.Seconds())
		if !allowed {
			retry = int(entry.started.Add(l.window).Sub(now).Seconds()) + 1
		}
		l.mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			problem(w, http.StatusTooManyRequests, "RATE_LIMITED")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isRateLimitedPath(path string) bool { return len(path) >= 10 && path[:10] == "/public/v1" }

func rateLimitKey(r *http.Request) string {
	if claims, ok := auth.ClaimsFrom(r.Context()); ok && claims.TenantID != "" {
		return "tenant:" + claims.TenantID
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return "ip:" + host
	}
	return "ip:" + r.RemoteAddr
}
