# ADR-PERSIST-001: Multi-replica persistence strategy

## Context

`MemoryRepository` serialises the entire domain snapshot as a single JSON blob on every mutation (`persistLocked()`). Under concurrent fiscal operations from multiple replicas sharing a Postgres-backed store, a naïve "last write wins" strategy can silently discard updates. Two approaches exist:

**Option A — per-tenant generation counter (current, extended)**  
Each tenant carries an integer `generation`. `SaveVersioned(b, expected)` rejects writes where the stored generation ≠ expected, forcing the losing replica to call `restoreLocked()` and retry. No cross-replica transactions are required; contention surfaces as retriable `ErrConcurrentState`. A `DeltaVersionedStore` variant sends only changed bytes when the previous snapshot is available.

**Option B — single-writer per tenant**  
A distributed lock (e.g., Postgres advisory lock or a Redis `SET NX`) ensures only one replica executes mutations for a given tenant at a time. Simpler to reason about, but adds lock-manager dependency and makes replicas active–standby rather than active–active. Undesirable for edge-routing scenarios where the IoT gateway may be the only live replica.

## Decision

**Use Option A (per-tenant generation counter)** for MVP1 and early production.

Rationale:
1. `SaveDeltaVersioned` already exists on the Postgres store and is exercised by `TestConcurrentFinalizeSameKeyFinalizeProducesOneOperation`; extending it to a full `PostgresRepository` requires no new infrastructure.
2. The fiscal protocol is inherently sequential per operation ID (`ReserveSaleFinalize` atomically gates all concurrent finalize attempts to a single writer); the generation counter is a safety net, not the primary guard.
3. A single-writer lock would require a lock manager not present in the current stack and would complicate failover during IoT connectivity gaps.

## Consequences

- `MemoryRepository` is the only repository implementation for MVP1; `MemoryRepository` backed by a `DeltaVersionedStore` is the path to multi-replica Postgres.
- If two replicas attempt a write in the same generation, one will fail and retry; this is observable as a warning log, not an error.
- `MemoryRepository` must be prohibited in `AppEnv != dev` (enforced in `cmd/fiscal-backend/main.go` via `TestMainRefusesMemoryRepoOutsideDev`).
- Snapshot size scales with total entity count; the 50 ms p95 threshold at N=10 000 sales is documented in `docs/MVP1/evidence/TG-001b.md`. A `persistLocked(touched ...string)` optimisation is deferred until N > 20 000.

## Status

Accepted — 2026-10-27.
