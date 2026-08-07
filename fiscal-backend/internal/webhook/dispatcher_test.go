package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fiscalisation/fiscal-backend/internal/domain"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSignedRetryDelivery(t *testing.T) {
	calls := 0
	rt := transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if !verifySignatureHeader(r.Header.Get("BeeFiscal-Signature"), body, "secret") {
			t.Error("canonical signature missing or invalid")
		}
		status := 204
		if calls == 1 {
			status = 503
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	svc := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	if e := svc.QueueFiscalEvent("sale-1", domain.Operation{ID: "op-1", State: "FISCALIZED", Version: 2}); e != nil {
		t.Fatal(e)
	}
	d := New(svc, "http://minipos.test/webhook", "secret")
	d.client = &http.Client{Transport: rt}
	now := time.Now().UTC()
	d.now = func() time.Time { return now }
	if e := d.DeliverOnce(context.Background()); e == nil {
		t.Fatal("first delivery must fail")
	}
	now = now.Add(11 * time.Second)
	if e := d.DeliverOnce(context.Background()); e != nil {
		t.Fatal(e)
	}
	if calls != 2 || len(svc.PendingOutbox(now.Add(time.Hour))) != 0 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestDynamicTenantEndpointDelivery(t *testing.T) {
	var expectedSecret string
	called := false
	rt := transportFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		body, _ := io.ReadAll(r.Body)
		if !verifySignatureHeader(r.Header.Get("BeeFiscal-Signature"), body, expectedSecret) || r.Header.Get("BeeFiscal-Event-Id") != "event-op-dynamic" {
			t.Error("dynamic endpoint signature or event ID mismatch")
		}
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	svc := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	created, err := svc.CreateWebhookEndpoint("tenant-a", map[string]any{"url": "https://hooks.example.test/fiscal", "events": []any{"fiscal.operation.updated"}})
	if err != nil {
		t.Fatal(err)
	}
	expectedSecret = created["secret"].(string)
	if err = svc.QueueFiscalEvent("sale-dynamic", domain.Operation{ID: "op-dynamic", TenantID: "tenant-a", State: "FISCALIZED", Version: 2}); err != nil {
		t.Fatal(err)
	}
	d := New(svc, "", "")
	d.client = &http.Client{Transport: rt}
	if err = d.DeliverOnce(context.Background()); err != nil || !called {
		t.Fatalf("dynamic delivery called=%v err=%v", called, err)
	}
}

func verifySignatureHeader(raw string, body []byte, secret string) bool {
	fields := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 {
			return false
		}
		fields[pair[0]] = pair[1]
	}
	if fields["t"] == "" || fields["kid"] == "" || fields["v1"] == "" {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(fields["t"] + "."))
	m.Write(body)
	return hmac.Equal([]byte(fields["v1"]), []byte(hex.EncodeToString(m.Sum(nil))))
}

func TestPerEndpointCheckpointAvoidsRepeatingSuccessfulDelivery(t *testing.T) {
	counts := map[string]int{}
	rt := transportFunc(func(r *http.Request) (*http.Response, error) {
		counts[r.URL.Host]++
		status := http.StatusNoContent
		if r.URL.Host == "b.example.test" && counts[r.URL.Host] == 1 {
			status = http.StatusServiceUnavailable
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	svc := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	for _, host := range []string{"a.example.test", "b.example.test"} {
		if _, err := svc.CreateWebhookEndpoint("tenant-a", map[string]any{"url": "https://" + host + "/fiscal", "events": []any{"fiscal.operation.updated"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.QueueFiscalEvent("sale-multi", domain.Operation{ID: "op-multi", TenantID: "tenant-a", State: "FISCALIZED", Version: 2}); err != nil {
		t.Fatal(err)
	}
	d := New(svc, "", "")
	d.client = &http.Client{Transport: rt}
	now := time.Now().UTC()
	d.now = func() time.Time { return now }
	if err := d.DeliverOnce(context.Background()); err == nil {
		t.Fatal("first pass must report endpoint B failure")
	}
	now = now.Add(11 * time.Second)
	if err := d.DeliverOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if counts["a.example.test"] != 1 || counts["b.example.test"] != 2 || len(svc.PendingOutbox(now.Add(time.Hour))) != 0 {
		t.Fatalf("unexpected delivery counts/state: %#v", counts)
	}
}

func TestFailureDoesNotSkipLaterEndpointAndGoneDisables(t *testing.T) {
	counts := map[string]int{}
	rt := transportFunc(func(r *http.Request) (*http.Response, error) {
		counts[r.URL.Host]++
		status := http.StatusServiceUnavailable
		if r.URL.Host == "gone.example.test" {
			status = http.StatusGone
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	svc := domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true))
	for _, host := range []string{"failed.example.test", "gone.example.test"} {
		if _, err := svc.CreateWebhookEndpoint("tenant-a", map[string]any{"url": "https://" + host + "/fiscal", "events": []any{"fiscal.operation.updated"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.QueueFiscalEvent("sale-independent", domain.Operation{ID: "op-independent", TenantID: "tenant-a", State: "FISCALIZED", Version: 2}); err != nil {
		t.Fatal(err)
	}
	d := New(svc, "", "")
	d.client = &http.Client{Transport: rt}
	if err := d.DeliverOnce(context.Background()); err == nil {
		t.Fatal("failed endpoint must keep outbox pending")
	}
	if counts["failed.example.test"] != 1 || counts["gone.example.test"] != 1 {
		t.Fatalf("endpoint delivery was not independent: %#v", counts)
	}
	if len(svc.WebhookDeliveryEndpoints("tenant-a", "fiscal.operation.updated")) != 1 {
		t.Fatal("HTTP 410 endpoint was not disabled")
	}
}

func TestRetryDelayMatchesContract(t *testing.T) {
	want := []time.Duration{10 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour}
	for i, v := range want {
		if got := retryDelay(i + 1); got != v {
			t.Fatalf("attempt %d got %s want %s", i+1, got, v)
		}
	}
	if retryDelay(99) != 24*time.Hour {
		t.Fatal("retry cap changed")
	}
}
