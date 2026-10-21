# Fiscal repository: recovery after uncertain persistence

Status: integrated software/fault tests and actual PostgreSQL regression PASS; physical interruption/failover acceptance remains open. Device PKI lifecycle work remains separate and open. Existing ESP32-S3 DeviceIdentity/NVS owns device keys; no ATECC608A/608 or new key export is introduced.

## Failure and required behavior

A Store Save may commit before returning an error. If the following Load also fails, speculative memory cannot be treated as committed, and an older acknowledged snapshot must never be written over a possibly committed fiscal fact. Previous immutable-resource and replay fast returns could publish that speculative state. Plain-store local rollback could also overwrite a successfully reloaded committed sale and UNP counter.

The repository remembers the last acknowledged snapshot on every successful plain/versioned/delta Save. Failed Save reloads authoritative state. If reload fails, returns corrupt structure, or loses an acknowledged collection, memory falls back to the confirmed snapshot and all authority reads/writes are fenced. A later successful refresh does not execute the triggering stale mutation: that request returns unavailable and must retry against refreshed state. No normal-path recovery Load is added.

Read-only methods retain RLocks. Direct tenant/system SQL readers check the same in-memory fence before and after IO. Errorless list/boolean interfaces return empty/false during uncertainty; these are unavailable results, not authoritative absence. Authenticated API business routes map recovery to retryable HTTP 503, including a failure arising after preflight. Background callers must retry rather than interpret empty lists as proof of absence. Normal known not-found behavior remains 404.

Local rollback records a restoration epoch, so it cannot undo a committed snapshot successfully reloaded during a failed Save. Uncertain UNP allocation returns no number/range; committed highwater survives recovery/restart. ClaimReplay cannot turn a speculative failed claim into cached success. Guards cover activation, resources, financial operations, delegated execution/reconciliation, edge sync and allocator paths before immutable/duplicate fast returns.

## Store contract and compatibility

Successful Load must return authoritative transaction-consistent state. Initial Load(nil,nil) is legitimate empty only before an acknowledged nonempty snapshot; versioned/delta additionally require generation zero. Synthetic empty memory during outage never establishes a nonempty baseline. Once a nonempty snapshot is acknowledged, empty reload is invalid.

Startup and recovery reject JSON null, empty/unrelated objects and wrong recognized collection types. Known sparse legacy objects remain supported. Recovery additionally requires every previously acknowledged collection to remain present. The historical optional edge_pending, activation_challenges and activation_requests may be absent, or null while absent/empty in the baseline; populated optional maps cannot disappear or become null. Other recognized collections must have map shape and audit must be an array. Constructor normalization initializes omitted sparse maps.

This is basic structural corruption detection, not proof against a Store lying with a structurally valid stale/full snapshot. Such reads violate the Store contract. No schema migration or device key migration is implied.

## Regression evidence and preliminary acceptance

Synthetic fault matrix covers plain, versioned and delta stores: definite failed Save, committed-then-error Save, recovery read outage, initial empty first write, healthy retry, restart, newer generation winning, immutable resource publication, ClaimReplay, actual CommitSaleOperation fiscal completion, activation entry, UNP continuity, parseable corruption and populated optional collection loss. Concurrent readers/recovery and a blocked direct SQL read race must pass the race detector. API tests require retryable 503 both before dispatch and after an in-request persistence failure.

Required integration checks: full fiscal-backend Go suite; domain/API race suite; actual PostgreSQL integration with existing immutable audit triggers unchanged. Production device/cloud interruptions, real database failover and all fiscal certification requirements remain outside synthetic acceptance. Secure BLE certificate issuance must wait for reviewed generic persistence behavior and fresh purpose-bound DeviceIdentity proof; existing clientAuth profile is unchanged.

## Complete pre-existing public repository surface (68 methods)

New `PersistenceReady` is an additional explicit service/API availability gate. Private lock-bearing helpers are also guarded: putResourceLocked, reserveSalePayment, reserveSaleReversal, commitResourceArtifactsOperationEvents, configureOfflineReconciliationV2. Table locations refer to this patch.

| Method | Location | Guard | Direct SQL read | Unavailable result |
|---|---|---|---|---|
| `PutActivationChallenge` | `activation.go:401` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ConsumeChallengeAndCreateActivation` | `activation.go:413` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ActivationRequest` | `activation.go:434` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | ErrRepositoryUnavailable |
| `ActivationRequestByCode` | `activation.go:449` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | ErrRepositoryUnavailable |
| `ConfirmActivationRequest` | `activation.go:465` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `MarkActivationCredentialIssued` | `activation.go:501` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ActivateDeviceRequest` | `activation.go:527` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `RevokeActivatedDevice` | `activation.go:593` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitDelegatedAuthorization` | `delegated_execution.go:193` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `BindOperatorIdentity` | `edge_sync_repository.go:167` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ReserveSaleFinalize` | `finalize_prepare.go:8` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ReserveUNPRange` | `repository.go:145` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `Replay` | `repository.go:534` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | zero/false |
| `ClaimReplay` | `repository.go:560` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `PutReplay` | `repository.go:600` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `PutAmbiguousReplay` | `repository.go:613` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `AddOutbox` | `repository.go:627` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `PendingOutbox` | `repository.go:638` | entry ensurePersistenceReady; RLock persistenceReadableLocked | system pre/post fence | empty slice |
| `UpdateOutbox` | `repository.go:672` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `PutBLESession` | `repository.go:681` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitBLESessionEvent` | `repository.go:690` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `BLESession` | `repository.go:702` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | ErrRepositoryUnavailable |
| `BLESessionForTenant` | `repository.go:717` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `LastSyncAck` | `repository.go:738` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | zero/false |
| `PutSyncAck` | `repository.go:765` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitEdgeSync` | `repository.go:774` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `EdgePendingCommand` | `repository.go:874` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `PutConnectivityProbe` | `repository.go:900` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ConnectivityProbe` | `repository.go:909` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | ErrRepositoryUnavailable |
| `ConnectivityProbeForTenant` | `repository.go:924` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `PutResource` | `repository.go:946` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitCompositeBinding` | `repository.go:1071` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `Resource` | `repository.go:1089` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `Resources` | `repository.go:1129` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | empty slice |
| `PutArtifact` | `repository.go:1162` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `Artifact` | `repository.go:1186` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `AuditEvents` | `repository.go:1231` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | empty slice |
| `AppendAudit` | `repository.go:1263` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `Sale` | `repository.go:1284` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | ErrRepositoryUnavailable |
| `SaleForTenant` | `repository.go:1299` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `Sales` | `repository.go:1320` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | empty slice |
| `OperationsForTenant` | `repository.go:1352` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | empty slice |
| `PutSale` | `repository.go:1380` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `OpenSaleWithFirstLine` | `repository.go:1408` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `AddSaleLineExpected` | `repository.go:1470` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ReplaceSaleLinesExpected` | `repository.go:1513` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `Operation` | `repository.go:1552` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | ErrRepositoryUnavailable |
| `OperationForTenant` | `repository.go:1567` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `Operations` | `repository.go:1588` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | empty slice |
| `PutOperation` | `repository.go:1603` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitOperationEvent` | `repository.go:1618` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ReserveSalePayment` | `repository.go:1638` | delegates to reserveSalePayment (mutex + persistenceReadyLocked) | — | ErrRepositoryUnavailable |
| `ReserveSalePaymentCommand` | `repository.go:1642` | delegates to reserveSalePayment (mutex + persistenceReadyLocked) | — | ErrRepositoryUnavailable |
| `ReserveSaleReversal` | `repository.go:1683` | delegates to reserveSaleReversal (mutex + persistenceReadyLocked) | — | ErrRepositoryUnavailable |
| `ReserveSaleReversalCommand` | `repository.go:1686` | delegates to reserveSaleReversal (mutex + persistenceReadyLocked) | — | ErrRepositoryUnavailable |
| `CommitSaleOperation` | `repository.go:1720` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitSaleOperationEvent` | `repository.go:1736` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitSaleOperationArtifact` | `repository.go:1758` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitSaleOperationArtifactEvent` | `repository.go:1790` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `CommitResourceArtifactsOperation` | `repository.go:1825` | delegates to commitResourceArtifactsOperationEvents (mutex + persistenceReadyLocked) | — | ErrRepositoryUnavailable |
| `CommitResourceArtifactsOperationEvents` | `repository.go:1829` | delegates to commitResourceArtifactsOperationEvents (mutex + persistenceReadyLocked) | — | ErrRepositoryUnavailable |
| `NextUNP` | `repository.go:1889` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `OpenShift` | `repository.go:1917` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `Shift` | `repository.go:1937` | entry ensurePersistenceReady; RLock persistenceReadableLocked | — | ErrRepositoryUnavailable |
| `ShiftForTenant` | `repository.go:1952` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | ErrRepositoryUnavailable |
| `Shifts` | `repository.go:1973` | entry ensurePersistenceReady; RLock persistenceReadableLocked | tenant pre/post fence | empty slice |
| `CloseShift` | `repository.go:2005` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |
| `ReserveFiscalUNPRange` | `unp_allocator.go:14` | existing mutex + persistenceReadyLocked | — | ErrRepositoryUnavailable |

Manual rollback epoch guards: OpenSaleWithFirstLine, CommitEdgeSync and ReserveFiscalUNPRange. Constructor cases and optional collection boundaries are exercised in repository_recovery_test.go; existing sparse legacy domain fixtures still pass.

## Follow-up: request recovery epochs and replay ownership

An unavailable observation must not become authoritative merely because a different caller completes recovery before this request writes its response. Each authenticated business request captures an in-process availability epoch after preflight. Both the outer response writer and an inner writer around the mutation router use that same epoch. The inner admission check rejects an already changed request epoch before invoking the router, including recovery completed during ClaimReplay. Any changed epoch converts the earlier observation to retryable 503 before the idempotency owner classifies/caches its captured result. Successful normal Save does not change this epoch; these checks add no database reads.

For middleware-owned requests, saveReplay/saveEmptyReplay only render the response; the middleware is the sole owner of final replay persistence. This prevents a handler from persisting a stale 404/empty result before the inner response fence sees it. Converted 503 retains a pending ambiguous replay, so the same key cannot execute the financial command again; callers must use status reconciliation. Standalone helper behavior remains unchanged. Error conversion strips stale ETag, Location, Content-Length and Content-Encoding.

Deterministic counterexamples fail against the previous API: stale observations return 200/404 after another caller recovers, and saveReplay/saveEmptyReplay finalize stale 404/204. New tests cover these interleavings through the production middleware and helper methods with a synthetic ServeMux fault route performing a real domain mutation; this is not a claim that every production route has a separate injected-fault test. Tests assert pending 503 survives restart, same-key retry does not dispatch again, body mismatch and malformed keys are rejected, ordinary successful body/empty replay and standalone helper persistence are preserved.

## Healthy multi-instance generation refresh

Request availability uses a separate availabilityEpoch. restorationEpoch still increments on every restore to protect manual rollback, including healthy remote-generation refresh. availabilityEpoch increments for failed Save/marshal recovery, pending recovery attempts, corrupt/failed authoritative reload and the first ClaimReplay LoadVersioned error. Healthy ClaimReplay refresh after a different instance's successful write does not invalidate a new request before business execution. The API method PersistenceEpoch exposes availabilityEpoch only.

A deterministic two-repository/shared VersionedStore test reproduces the previous false 503: remote healthy resource write advances generation, then a new POST on the stale local repository refreshes during ClaimReplay. The corrected path executes exactly once, persists final 201, replays normally and adopts remote state; it does not create a permanent pending ambiguity for an unexecuted ordinary command. Existing failed-save/recovery epoch counterexamples remain covered. A separate domain regression requires ClaimReplay authoritative read outage to fence reads without Save and recover through explicit retry.

## Root integration evidence

[Manifest and raw logs](evidence/fiscal-repository-recovery-2026-10-21/manifest.json): full Fiscal backend, domain/API race and actual PostgreSQL integration PASS after the final healthy-refresh correction. RED logs retain the stale-response/replay and healthy multi-instance counterexamples. PostgreSQL tests validate ordinary canonical persistence/audit behavior; uncertain Save/COMMIT injection is synthetic, and real failover remains an external runtime test. New availability accessors supplement the 68-method inventory; normal reads add no database reload.
