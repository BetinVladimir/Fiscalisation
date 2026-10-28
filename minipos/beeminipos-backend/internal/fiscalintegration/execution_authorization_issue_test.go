package fiscalintegration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// buildValidEnvelope returns a response that passes all Issue validations.
func buildValidEnvelope(t *testing.T, tenant, appInstanceID, clientKey, algorithm string, expiresAt time.Time) ExecutionAuthorizationResponse {
	t.Helper()
	payloadClaims := map[string]any{
		"tenant_id":                           tenant,
		"app_instance_id":                     appInstanceID,
		"client_signing_public_key_der_base64": clientKey,
		"expires_at":                          expiresAt.UTC().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(payloadClaims)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256(raw)
	return ExecutionAuthorizationResponse{
		Authorization: ExecutionAuthorizationEnvelope{
			Algorithm: algorithm,
			KeyID:     "key-1",
			Payload:   encoded,
			SHA256:    hex.EncodeToString(sum[:]),
			Signature: "sig",
		},
	}
}

func issueRT(statusCode int, body string) readTransport {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}
}

func TestIssueRejectsShaMismatch(t *testing.T) {
	resp := buildValidEnvelope(t, "fiscal-tenant-a", "app-1", "cGVtLWtleQ", "ES256", time.Now().Add(time.Hour))
	resp.Authorization.SHA256 = strings.Repeat("0", 64)
	body, _ := json.Marshal(resp)

	c := dispatchClient(t, issueRT(200, string(body)))
	_, err := c.IssueExecutionAuthorization(context.Background(), "company-1",
		ExecutionAuthorizationRequest{AppInstanceID: "app-1", ClientSigningPublicKeyDERBase64: "cGVtLWtleQ"},
		"idem-issue-sha256-001")
	if err == nil {
		t.Fatal("expected error for SHA256 mismatch, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "sha256") {
		t.Fatalf("expected sha256 error message, got %v", err)
	}
}

func TestIssueRejectsExpiredAuthorization(t *testing.T) {
	resp := buildValidEnvelope(t, "fiscal-tenant-a", "app-1", "cGVtLWtleQ", "ES256", time.Now().Add(-5*time.Minute))
	body, _ := json.Marshal(resp)

	c := dispatchClient(t, issueRT(200, string(body)))
	_, err := c.IssueExecutionAuthorization(context.Background(), "company-1",
		ExecutionAuthorizationRequest{AppInstanceID: "app-1", ClientSigningPublicKeyDERBase64: "cGVtLWtleQ"},
		"idem-issue-expiry-001")
	if err == nil {
		t.Fatal("expected error for expired authorization, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "expir") {
		t.Fatalf("expected expiry error message, got %v", err)
	}
}

func TestIssueRejectsUnknownAlgorithm(t *testing.T) {
	resp := buildValidEnvelope(t, "fiscal-tenant-a", "app-1", "cGVtLWtleQ", "RS1", time.Now().Add(time.Hour))
	body, _ := json.Marshal(resp)

	c := dispatchClient(t, issueRT(200, string(body)))
	_, err := c.IssueExecutionAuthorization(context.Background(), "company-1",
		ExecutionAuthorizationRequest{AppInstanceID: "app-1", ClientSigningPublicKeyDERBase64: "cGVtLWtleQ"},
		"idem-issue-algo-001")
	if err == nil {
		t.Fatal("expected error for unknown algorithm, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "algorithm") {
		t.Fatalf("expected algorithm error message, got %v", err)
	}
}
