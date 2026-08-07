package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitRejectsBurstAndRecoversAtWindowBoundary(t *testing.T) {
	now := time.Unix(100, 0)
	l := &requestLimiter{limit: 2, window: time.Minute, now: func() time.Time { return now }, entries: map[string]rateWindow{}}
	h := l.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := func(path, remote string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = remote
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if request("/public/v1/sales", "192.0.2.1:1000").Code != 204 || request("/public/v1/sales", "192.0.2.1:2000").Code != 204 {
		t.Fatal("allowed burst rejected")
	}
	blocked := request("/public/v1/sales", "192.0.2.1:3000")
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d", blocked.Code)
	}
	if request("/healthz", "192.0.2.1:4000").Code != 204 {
		t.Fatal("health endpoint must not be rate limited")
	}
	now = now.Add(time.Minute)
	if request("/public/v1/sales", "192.0.2.1:5000").Code != 204 {
		t.Fatal("limit did not recover at window boundary")
	}
}
