package domain

import (
	"context"
	"sync/atomic"
	"time"
)

const (
	reconcilerDefaultInterval = 30 * time.Second
	reconcilerDefaultStaleness = 5 * time.Minute
	ReconcilerMaxAttempts      = 3
)

// StuckOperationsCounter is the fiscal_operations_stuck metric.
var StuckOperationsCounter atomic.Int64

// ReconcilerRepo is the minimal read/write surface the Reconciler needs.
// It is a subset of Repository, allowing test doubles without the full interface.
type ReconcilerRepo interface {
	Operations() []Operation
	PutOperation(Operation) error
}

// Reconciler periodically promotes EXECUTING/UNKNOWN operations that have
// been stuck longer than StalenessThreshold into a terminal state or escalates
// them after MaxAttempts.
type Reconciler struct {
	repo               ReconcilerRepo
	interval           time.Duration
	StalenessThreshold time.Duration
	MaxAttempts        int
	escalate           func(op Operation)
	now                func() time.Time
}

// NewReconciler returns a Reconciler with production defaults.
func NewReconciler(repo ReconcilerRepo) *Reconciler {
	return &Reconciler{
		repo:               repo,
		interval:           reconcilerDefaultInterval,
		StalenessThreshold: reconcilerDefaultStaleness,
		MaxAttempts:        ReconcilerMaxAttempts,
		escalate:           func(op Operation) {},
		now:                time.Now,
	}
}

// Run starts the periodic reconciliation loop; it blocks until ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Tick()
		}
	}
}

// Tick runs one reconciliation pass (exported for testing without a goroutine).
func (r *Reconciler) Tick() {
	stuck := r.FindStuckOperations()
	StuckOperationsCounter.Store(int64(len(stuck)))
	for _, op := range stuck {
		r.ReconcileOperation(op)
	}
}

// FindStuckOperations returns operations in EXECUTING or UNKNOWN state that
// have not been updated within StalenessThreshold.
func (r *Reconciler) FindStuckOperations() []Operation {
	all := r.repo.Operations()
	cutoff := r.now().Add(-r.StalenessThreshold)
	var out []Operation
	for _, op := range all {
		if (op.State == "EXECUTING" || op.State == "UNKNOWN") && op.UpdatedAt.Before(cutoff) {
			out = append(out, op)
		}
	}
	return out
}

// ReconcileOperation moves one stuck operation toward a terminal state.
// It increments an internal attempt counter stored in ErrorCode prefix.
// After MaxAttempts the operation is escalated and moved to FAILED.
// While attempts remain, it stays UNKNOWN so no monetary side-effect fires.
func (r *Reconciler) ReconcileOperation(op Operation) {
	attempts := countReconcileAttempts(op)
	if attempts >= r.MaxAttempts {
		op.State = "FAILED"
		op.ErrorCode = "RECONCILE_ESCALATED"
		op.AllowedActions = []string{}
		op.UpdatedAt = r.now()
		_ = r.repo.PutOperation(op) // best-effort: escalate regardless; storage failure is logged upstream
		r.escalate(op)
		return
	}
	// Record attempt; keep UNKNOWN (invariant 2: UNKNOWN ≠ FAILED — no refund here).
	op.ErrorCode = buildReconcileAttemptCode(attempts + 1)
	op.State = "UNKNOWN"
	op.AllowedActions = []string{"RECONCILE"}
	op.UpdatedAt = r.now()
	_ = r.repo.PutOperation(op) // best-effort: UNKNOWN state persisted if possible; invariant 2 holds regardless
}

// countReconcileAttempts reads the attempt number embedded in ErrorCode.
func countReconcileAttempts(op Operation) int {
	// Format: "RECONCILE_ATTEMPT_N"
	const prefix = "RECONCILE_ATTEMPT_"
	if len(op.ErrorCode) <= len(prefix) {
		return 0
	}
	n := 0
	for _, ch := range op.ErrorCode[len(prefix):] {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func buildReconcileAttemptCode(n int) string {
	const prefix = "RECONCILE_ATTEMPT_"
	// n is at most MaxAttempts (3), so simple itoa is fine.
	digits := []byte{'0' + byte(n%10)}
	if n >= 10 {
		digits = append([]byte{'0' + byte(n/10)}, digits...)
	}
	return prefix + string(digits)
}
