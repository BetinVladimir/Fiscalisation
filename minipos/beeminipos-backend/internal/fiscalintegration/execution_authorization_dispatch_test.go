package fiscalintegration

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func dispatchClient(t *testing.T, rt http.RoundTripper) *Client {
	t.Helper()
	block, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(block)
	c := &Client{base: "https://fiscal.example", aead: aead}
	encrypted, _ := c.encrypt("tenant_live_cred-abc.secret")
	d := &recoveryDriver{encrypted: encrypted}
	db := sql.OpenDB(recoveryConnector{d})
	t.Cleanup(func() { db.Close() })
	c.db = db
	c.http = &http.Client{Transport: rt}
	return c
}

func TestDispatchSendsDeterministicIdempotencyKey(t *testing.T) {
	var capturedKey string
	rt := readTransport(func(r *http.Request) (*http.Response, error) {
		capturedKey = r.Header.Get("Idempotency-Key")
		body := `{"operation_id":"op-abc","client_operation_id":"op-abc","tenant_id":"fiscal-tenant-a","state":"ACCEPTED","allowed_actions":["READ"]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	c := dispatchClient(t, rt)
	_, err := c.DispatchExecutionAuthorization(context.Background(), "company-one", "auth-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if capturedKey != "dispatch-auth-xyz" {
		t.Fatalf("expected Idempotency-Key=dispatch-auth-xyz, got %q", capturedKey)
	}
}

func TestDispatchNetworkErrorReturnsErrDispatchOutcomeUnknown(t *testing.T) {
	rt := readTransport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: connection refused")
	})
	c := dispatchClient(t, rt)
	_, err := c.DispatchExecutionAuthorization(context.Background(), "company-one", "auth-xyz")
	if !errors.Is(err, ErrDispatchOutcomeUnknown) {
		t.Fatalf("expected ErrDispatchOutcomeUnknown, got %v", err)
	}
}
