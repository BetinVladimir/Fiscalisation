package domain

import (
	"testing"
	"time"
)

// memReconcilerRepo is a minimal in-memory ReconcilerRepo for tests.
type memReconcilerRepo struct {
	ops map[string]Operation
}

func newMemReconcilerRepo(ops ...Operation) *memReconcilerRepo {
	r := &memReconcilerRepo{ops: make(map[string]Operation)}
	for _, op := range ops {
		r.ops[op.ID] = op
	}
	return r
}

func (r *memReconcilerRepo) Operations() []Operation {
	out := make([]Operation, 0, len(r.ops))
	for _, op := range r.ops {
		out = append(out, op)
	}
	return out
}

func (r *memReconcilerRepo) PutOperation(op Operation) error {
	r.ops[op.ID] = op
	return nil
}

func staleOp(id, state string, updatedAgo time.Duration) Operation {
	t := time.Now().Add(-updatedAgo)
	return Operation{
		ID:             id,
		TenantID:       "tenant-1",
		RegisterID:     "reg-1",
		Type:           "SALE",
		State:          state,
		Version:        1,
		AllowedActions: []string{"RECONCILE"},
		CreatedAt:      t,
		UpdatedAt:      t,
	}
}

func TestReconcilerResolvesExecutingOperations(t *testing.T) {
	stale := staleOp("op-exec-1", "EXECUTING", 10*time.Minute)
	recent := staleOp("op-exec-2", "EXECUTING", 1*time.Minute)
	repo := newMemReconcilerRepo(stale, recent)

	rec := NewReconciler(repo)
	rec.StalenessThreshold = 5 * time.Minute

	stuck := rec.FindStuckOperations()
	if len(stuck) != 1 {
		t.Fatalf("FindStuckOperations: want 1 stuck op, got %d", len(stuck))
	}
	if stuck[0].ID != "op-exec-1" {
		t.Fatalf("wrong op returned: %s", stuck[0].ID)
	}

	rec.Tick()

	// op-exec-1 must have been updated (attempt recorded)
	updated := repo.ops["op-exec-1"]
	if updated.State != "UNKNOWN" {
		t.Errorf("op-exec-1 state: want UNKNOWN, got %s", updated.State)
	}
	if updated.ErrorCode != "RECONCILE_ATTEMPT_1" {
		t.Errorf("op-exec-1 error_code: want RECONCILE_ATTEMPT_1, got %s", updated.ErrorCode)
	}

	// op-exec-2 (recent) must be untouched
	untouched := repo.ops["op-exec-2"]
	if untouched.State != "EXECUTING" {
		t.Errorf("op-exec-2 should not have been reconciled, state=%s", untouched.State)
	}
}

func TestReconcilerEscalatesAfterMaxAttempts(t *testing.T) {
	op := staleOp("op-unk-1", "UNKNOWN", 10*time.Minute)
	op.ErrorCode = "RECONCILE_ATTEMPT_3" // already at MaxAttempts

	repo := newMemReconcilerRepo(op)

	escalated := false
	rec := NewReconciler(repo)
	rec.StalenessThreshold = 5 * time.Minute
	rec.escalate = func(o Operation) { escalated = true }

	rec.ReconcileOperation(op)

	result := repo.ops["op-unk-1"]
	if result.State != "FAILED" {
		t.Errorf("after MaxAttempts: want state FAILED, got %s", result.State)
	}
	if result.ErrorCode != "RECONCILE_ESCALATED" {
		t.Errorf("after MaxAttempts: want error_code RECONCILE_ESCALATED, got %s", result.ErrorCode)
	}
	if !escalated {
		t.Error("escalate callback must be called after MaxAttempts")
	}
	// Invariant 2: UNKNOWN ≠ FAILED — FAILED with RECONCILE_ESCALATED must have no RECONCILE action
	for _, a := range result.AllowedActions {
		if a == "RECONCILE" {
			t.Error("escalated operation must not have RECONCILE in AllowedActions")
		}
	}
}
