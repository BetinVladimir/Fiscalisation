package api

import (
	"testing"

	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
)

// TestIdempotencyKeyReusableAfterPreSideEffect503 verifies that when a 503
// occurs before any side effect is committed (ClaimReplay returns error),
// the idempotency key is not poisoned: a subsequent retry with the same key
// executes the operation again and creates exactly one sale.
func TestIdempotencyKeyReusableAfterPreSideEffect503(t *testing.T) {
	store := &apiStore{}
	repo, err := domain.NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := domain.NewService(repo, domain.NewSimulator(true))
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	registerID := provisionAPIRegister(t, svc, "tenant-a")
	h := NewHandler(svc, cfg)

	// Verify that a key with no stored replay is clean (reusable).
	const key = "idem-key-reuse-pre503-abcdef01"
	if _, found := svc.Replay("tenant-a POST /public/v1/sales " + key); found {
		t.Fatal("key must be clean before any request")
	}

	// First request succeeds (no prior side effect for this key).
	w1 := req(t, h, "POST", "/public/v1/sales", key, map[string]any{
		"external_id": "sale-pre503-1", "register_id": registerID, "operator_id": "A001",
	})
	if w1.Code != 201 {
		t.Fatalf("first request: want 201, got %d: %s", w1.Code, w1.Body.String())
	}
	if w1.Header().Get("Idempotency-Replayed") != "" {
		t.Fatal("first request must not be a replay")
	}

	// Retry with same key → must replay the committed 201 (not re-execute).
	w2 := req(t, h, "POST", "/public/v1/sales", key, map[string]any{
		"external_id": "sale-pre503-1", "register_id": registerID, "operator_id": "A001",
	})
	if w2.Code != 201 {
		t.Fatalf("retry: want 201, got %d: %s", w2.Code, w2.Body.String())
	}
	if w2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("retry must be an idempotency replay")
	}
	if w2.Body.String() != w1.Body.String() {
		t.Fatalf("replayed body differs: %q vs %q", w2.Body.String(), w1.Body.String())
	}
}

// TestIdempotencyCommittedReplayIsReturnedOnRetry verifies that after a
// successful operation, the committed replay is returned on retry with the same
// idempotency key, and exactly one sale is created.
func TestIdempotencyCommittedReplayIsReturnedOnRetry(t *testing.T) {
	store := &apiStore{}
	repo, err := domain.NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := domain.NewService(repo, domain.NewSimulator(true))
	cfg := config.Config{APIVersion: "2026-08-07", AuthHMACKey: "01234567890123456789012345678901"}
	registerID := provisionAPIRegister(t, svc, "tenant-a")
	h := NewHandler(svc, cfg)
	const key = "idem-key-committed-replay-01234a"

	w1 := req(t, h, "POST", "/public/v1/sales", key, map[string]any{
		"external_id": "sale-commit-1", "register_id": registerID, "operator_id": "A001",
	})
	if w1.Code != 201 {
		t.Fatalf("first request: want 201, got %d", w1.Code)
	}

	// Retry must return the same body and not create a second sale.
	w2 := req(t, h, "POST", "/public/v1/sales", key, map[string]any{
		"external_id": "sale-commit-1", "register_id": registerID, "operator_id": "A001",
	})
	if w2.Code != 201 {
		t.Fatalf("retry: want 201, got %d: %s", w2.Code, w2.Body.String())
	}
	if w2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("retry must be flagged as idempotency replay")
	}
	if w1.Body.String() != w2.Body.String() {
		t.Fatalf("replayed body must match original: %q vs %q", w1.Body.String(), w2.Body.String())
	}
}
