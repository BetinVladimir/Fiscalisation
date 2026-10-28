# Persistent terminal quarantine and reconciliation contract

2026-10-18. Scope: actual S3 `ProfileExecutor` / SQLite journal; no Go edge-agent. Internal ESP32-S3/NVS keys remain the hardware configuration.

## Discovered gap and protection

The existing saga checkpoints CARD_SENDING before I/O and CARD_UNKNOWN afterwards, and restart recovery refuses another purchase for the **same operation**. Its original execute path did not inspect another operation's uncertain payment. A new operation on the same BluePad after process restart could therefore purchase despite a previous unresolved outcome. The original actual-runtime restart regression is RED: its second operation sends a payment.

`terminal_payment_guard` now records the immutable payment-terminal device ID and owning operation in the same BEGIN IMMEDIATE transaction that creates the payment saga/membership. This happens before CARD_SENDING and any physical purchase. There is at most one owner per terminal. Competing SQLite connections serialize ownership; a failed saga checkpoint rolls the guard back. A different operation receives `PAYMENT_TERMINAL_RECONCILIATION_REQUIRED` before terminal I/O. Cash and another terminal remain usable.

The guard survives unknown results, send/checkpoint failures, power loss and process restart. It also remains for an approved bank payment whose fiscalization or compensation is incomplete. There is no clock-based expiration and no public clear command. New guard acquisition may replace an old guard only after all original CARD members agree with the completed business result:

| Operation | Required payment states |
|---|---|
| COMMITTED | Every CARD member CARD_APPROVED |
| COMPENSATED | Every member REVERSED / CARD_DECLINED / CARD_NOT_SENT / RESERVED |
| REJECTED | Every member CARD_DECLINED / CARD_NOT_SENT / RESERVED |

An operation marked COMMITTED while a member is CARD_UNKNOWN cannot release the guard. An approved payment followed by a REJECTED operation cannot release it either. Reversal failures/unknowns remain blocked. The guard retains its owner until another safe acquisition, so historical evidence is available even after successful completion.

Pre-upgrade databases are checked for unresolved legacy payment rows using each operation's immutable `route_snapshot.payment_device_id`; the current mutable binding is never used to guess an old terminal. An unresolved legacy row lacking terminal identity returns `PAYMENT_TERMINAL_LEGACY_INVENTORY_REQUIRED` for CARD: authorized historical inventory/reconciliation is necessary. Cash remains available. A terminal resource ID must remain stable for the same physical terminal; re-registering the same hardware under a new resource ID is not a reconciliation method. Backend onboarding/resource alias controls remain part of the ADMIN followup.

## Typed resolution core

`reconcile_payment(expected, observed, next_state, now, evidence)` retains the existing full-row CAS, original purchase identity, legal state transitions and saga-phase checks. Optional `PaymentReconciliationEvidence` is for a **trusted verified adapter**, not an authorization verifier. It requires reconciliation ID, exact guarded terminal/operation, actor, immutable external evidence reference, evidence SHA256 and authorization SHA256. A bank approval additionally requires STAN/reference, RRN and AUTH. Existing verified read-only query recovery remains subject to its false-by-default capability flag; no new banking query or reversal is enabled.

When evidence is supplied, payment update, immutable `payment_reconciliations` record and `PAYMENT_RECONCILED` outbox event commit in one transaction. The event includes exact operation/payment/terminal, old/new states, decimal-string amount, original and reversal references, actor and evidence/authorization hashes. Stale expected rows, wrong ownership/amount, evidence errors, duplicate IDs or conflicting existing outbox events roll everything back. Approval changes the payment state but does not delete the guard or declare a fiscal outcome. Normal fiscal proof / verified compensation and final result projection are still required.

No unauthenticated ADMIN endpoint, file edit or reset shortcut is exposed. The optional null terminal argument in the low-level legacy journal API supports historical/import/test callers; actual production `ProfileExecutor` always requires and supplies `binding.payment.device_id` before purchasing.

## Signed ADMIN reconciliation wire contract — OPEN until wired authorization

This is the required followup contract, not an already enabled command:

- Kind `PAYMENT_RECONCILE`, version1, signed server-issued authorization targeted to tenant/register/edge device, historical binding SHA256+generation and terminal ID. Existing authenticated transport alone must not confer ADMIN authority on an arbitrary local caller.
- Authorization includes unique reconciliation ID, authenticated ADMIN actor, immutable acquirer case/evidence reference+SHA256, issue/expiry bounds, exact operation/payment/receipt IDs, exact amount_minor and full expected payment row (state and all original/reversal references). A Go-canonical expected-row SHA256 may bind the full row but must not replace identity/amount checks.
- Observed result includes Purchase/Reversal kind, proved outcome and all applicable STAN/RRN/AUTH fields. Approved bank payment is not proof of fiscal completion. Declined/NotSent/REVERSED require authoritative evidence for this exact payment; a missing report or timeout is insufficient absence proof.
- Firmware verifies issuer trust, ADMIN scope, historical context, current terminal ownership, time window, replay ID, expected-row CAS and evidence hash before calling the typed core. Local BLE/HTTP strings cannot invoke it without equivalent verified authorization.
- Backend preserves issued authorization and evidence, consumes `PAYMENT_RECONCILED` as an immutable audit event, and exposes pending fiscal/compensation work. The operator cannot mark a payment resolved merely to unlock the terminal.
- Replays return the same durable disposition or a clear stale/conflict response; they never cause new bank commands. Guard release follows the table above after the business saga completes.

**OPEN:** backend ADMIN issuance/roles, cryptographic wire verification/dispatch, backend audit-event acceptance, immutable physical-terminal alias policy, and authenticated operator UI. Parent assigns these after the backend bridge integration. Until those are wired, unresolved BluePad payments remain blocked and require external acquirer/manual investigation; this package does not claim an available remote unlock.

## Tests and preliminary acceptance

`python3 IoT/firmware/edge-agent-s3/idf/tests/terminal_guard_host/run.py` compiles actual production storage/orchestrator against real SQLite and exercises restart and two-connection concurrency. Cases include same/other terminal, cash, missing legacy identity, pre-upgrade rows, interrupted CARD_SENDING, outcome/saga failures, successful completed release, approved/fiscal-open retention, contradictory final operation state, audited decline resolution, approval without release, wrong terminal/amount/evidence, stale CAS, outbox collision and failure rollback. Existing saga and tender-recovery suites must remain green; actual pinned ESP32-S3 compilation must pass.

HIL acceptance additionally cuts power before/after every purchase/checkpoint, restarts S3 with the same SD journal, verifies no new bank command on a different operation for the locked terminal, confirms other terminal/cash availability, and proves the eventual ADMIN workflow cannot discard fiscal/compensation obligations. Query/reversal capabilities remain false until independently documented and certified.
