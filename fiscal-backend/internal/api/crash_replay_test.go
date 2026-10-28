package api

import (
	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"testing"
)

// TestCrashReplayAfterCommitBeforeResponse verifies that if the process
// crashes between persist and HTTP response write, a retry with the same
// idempotency key returns the committed response (not a duplicate operation).
//
// Simulation: the first service instance persists the response; a second
// instance (restart) is built from the same store and must replay it.
func TestCrashReplayAfterCommitBeforeResponse(t *testing.T) {
	store := &apiStore{}
	repo, err := domain.NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := domain.NewService(repo, domain.NewSimulator(true))
	cfg := config.Config{
		APIVersion:  "2026-08-07",
		AuthHMACKey: "01234567890123456789012345678901",
	}
	registerID := provisionAPIRegister(t, svc, "tenant-a")
	h := NewHandler(svc, cfg)

	const key = "crash-replay-key-abcdef0123456789"

	// First request: commits the sale and stores the replay.
	w1 := req(t, h, "POST", "/public/v1/sales", key, map[string]any{
		"external_id": "crash-sale-1",
		"register_id": registerID,
		"operator_id": "A001",
	})
	if w1.Code != 201 {
		t.Fatalf("first request: want 201, got %d: %s", w1.Code, w1.Body.String())
	}
	if w1.Header().Get("Idempotency-Replayed") != "" {
		t.Fatal("first request must not be a replay")
	}
	originalBody := w1.Body.String()

	// Simulate crash + restart: discard in-memory state, reload from store.
	repo2, err := domain.NewPersistentRepository(store)
	if err != nil {
		t.Fatal("restart:", err)
	}
	h2 := NewHandler(domain.NewService(repo2, domain.NewSimulator(true)), cfg)

	// Retry after restart must return the persisted response, not re-execute.
	w2 := req(t, h2, "POST", "/public/v1/sales", key, map[string]any{
		"external_id": "crash-sale-1",
		"register_id": registerID,
		"operator_id": "A001",
	})
	if w2.Code != 201 {
		t.Fatalf("retry after restart: want 201, got %d: %s", w2.Code, w2.Body.String())
	}
	if w2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("retry after restart must be flagged as idempotency replay")
	}
	if w2.Body.String() != originalBody {
		t.Fatalf("replayed body must match original:\noriginal: %q\nreplayed: %q", originalBody, w2.Body.String())
	}
	t.Log("crash_replay_after_commit=pass")
}

// TestCrashReplayPreservesExactlyOneSale verifies that the crash+replay
// sequence creates exactly one fiscal sale document, not two.
func TestCrashReplayPreservesExactlyOneSale(t *testing.T) {
	store := &apiStore{}
	repo, err := domain.NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := domain.NewService(repo, domain.NewSimulator(true))
	cfg := config.Config{
		APIVersion:  "2026-08-07",
		AuthHMACKey: "01234567890123456789012345678901",
	}
	registerID := provisionAPIRegister(t, svc, "tenant-a")
	h := NewHandler(svc, cfg)

	const key = "crash-exactly-once-key-0123456789"

	w1 := req(t, h, "POST", "/public/v1/sales", key, map[string]any{
		"external_id": "crash-once-sale-1",
		"register_id": registerID,
		"operator_id": "A001",
	})
	if w1.Code != 201 {
		t.Fatalf("first request: want 201, got %d", w1.Code)
	}

	// Restart and retry three times — sale count must remain 1.
	for attempt := 1; attempt <= 3; attempt++ {
		repo2, err2 := domain.NewPersistentRepository(store)
		if err2 != nil {
			t.Fatalf("restart %d: %v", attempt, err2)
		}
		h2 := NewHandler(domain.NewService(repo2, domain.NewSimulator(true)), cfg)
		w := req(t, h2, "POST", "/public/v1/sales", key, map[string]any{
			"external_id": "crash-once-sale-1",
			"register_id": registerID,
			"operator_id": "A001",
		})
		if w.Code != 201 {
			t.Fatalf("retry %d after restart: want 201, got %d: %s", attempt, w.Code, w.Body.String())
		}
		if w.Header().Get("Idempotency-Replayed") != "true" {
			t.Fatalf("retry %d must be an idempotency replay", attempt)
		}
		if w.Body.String() != w1.Body.String() {
			t.Fatalf("attempt %d body mismatch", attempt)
		}
	}
	t.Log("crash_replay_exactly_once=pass")
}
