package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitRejectsBurstAndLeavesPrivateWebhookUnthrottled(t *testing.T) {
	now := time.Unix(100, 0)
	l := &requestLimiter{limit: 1, window: time.Minute, now: func() time.Time { return now }, entries: map[string]rateWindow{}}
	h := l.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	call := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = "192.0.2.2:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if call("/public/v1/minipos/products").Code != 204 {
		t.Fatal("first request rejected")
	}
	if w := call("/public/v1/minipos/products"); w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("expected rate limit, got %d", w.Code)
	}
	if call("/api/v1/fiscal-webhooks").Code != 204 {
		t.Fatal("signed internal webhook unexpectedly rate limited")
	}
	now = now.Add(time.Minute)
	if call("/public/v1/minipos/products").Code != 204 {
		t.Fatal("window did not reset")
	}
}
