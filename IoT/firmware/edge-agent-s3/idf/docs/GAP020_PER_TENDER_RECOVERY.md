# GAP019/020: recovery of frozen CARD tender inventories

This package replaces the blanket new-saga recovery block. It does not declare physical CARD readiness.

## Durable authority and state machine

`payment_saga_snapshot` reads the saga phase, owning nonterminal operation, and **all frozen members** in one SQLite read transaction. Receipt ownership, positive amounts and member rows are mandatory. Terminal operations are not queried or mutated. Recovery never treats `RecoveryItem`'s historically selected single payment as the whole sale.

- `COMPENSATED` plus consistent per-tender terminal states returns `Compensated` without IO. A lost final result/outbox projection can therefore be rebuilt through existing atomic `complete_operation`.
- `PAYMENTS` containing only `RESERVED`, proven `CARD_DECLINED` or `CARD_NOT_SENT` returns `Rejected` without IO. A reserved tender has never passed the durable before-purchase checkpoint.
- `CARD_SENDING` / `CARD_UNKNOWN` and `REVERSAL_SENDING` / `REVERSAL_UNKNOWN` require an explicitly verified, read-only, scoped terminal query. Missing capability, uncertain result, wrong operation/payment/amount or contradictory purchase identity preserves `Unknown`.
- Query observations use `reconcile_payment`: one SQLite transaction compares the complete previously observed row, frozen membership, operation ownership and phase; updates the individual tender/reversal evidence and recovery state atomically. Concurrent changed refs or state cannot be overwritten.
- Recovery never calls `purchase`. It never resends a reversal whose intent or uncertain outcome exists. A failed reversal query remains manual reconciliation; retry policy is not invented.
- New compensation may begin only after every sibling is known, and there is a proven failed/not-sent/unattempted path or a prior compensation phase. It requires verified reversal capability and every original approved RRN. Each new reversal uses the existing before/after checkpoints; original purchase refs remain distinct from reversal refs.
- `FISCAL_SENDING` calls only `fiscal_receipt_proof`, never legacy mutating `recover`. `Committed` requires exact physical receipt proof and every CARD tender durably approved. `Rejected` from that API means **proven absence of this exact operation**, not a timeout, current closed receipt or arbitrary latest receipt. Only that proof permits compensation. All-approved `PAYMENTS` without fiscal intent is left `Unknown` pending an explicit safe continuation policy; it never invents or resends fiscal issuance.
- Legacy CARD inventories cannot use legacy recovery's selected one-payment/cancel/reverse path. They return `LEGACY_CARD_INVENTORY_REQUIRED`. Invalid legacy canonical payload also fails closed. Cash-only legacy recovery remains the prior path.

`RECOVERY_REQUIRED` does not enable normal purchase transitions. Only the dedicated reconciliation transaction or a CAS from durable `CARD_APPROVED` into **new** `REVERSAL_SENDING` accepts that recovery ownership. Terminal owners remain immutable.

## Capability contracts and remaining external work

`DeviceIo::query_payment(PaymentQuery)` is read-only and must prove the exact operation/payment identity, amount and original stored references. For a reversal query the query includes the original RRN and distinct durable reversal evidence. `NotSent` must prove absence, not mean "not found in an incomplete history window". A verified adapter is responsible for this vendor-supported identity proof; echoing request fields is insufficient evidence in an actual implementation.

Actual `EspIdfDeviceIo` leaves `verified_payment_query_profile` and `verified_reversal_profile` **false**. The BORICA/BluePad query/reference/reversal contract has not been established from authoritative vendor material. Software adapters explicitly enable those capabilities solely to test orchestration and fault boundaries. This is temporarily incomplete physical functionality, not CARD HIL readiness. Obtain the protocol, exact terminal transaction identity and retention/absence semantics; add independent vendor command/response vectors and hardware evidence before enabling them.

The `fiscal_receipt_proof` declaration is coordinated with the separate GAP021 vendor readout package. This patch alone intentionally does not implement its real adapter override; integrate GAP021 together before linking actual `EspIdfDeviceIo` targets. Receipt identity must distinguish sale/storno and include exact UNP/document metadata.

Existing backend result values remain `COMMITTED`, `COMPENSATED`, `REJECTED`, `RECOVERY_REQUIRED`; no new wire state or operation-level payment-ref authority is introduced. Individual tender evidence remains in SQLite. A frontend/operator reconciliation workflow, legacy inventory migration, safe continuation for all-approved-before-fiscal-intent, physical CARD refund original-reference request contract and verified terminal adapter remain outstanding. GAP019/020 cannot be declared fully hardware-ready merely because these software tests pass.

## Executable acceptance

Run `bash tests/run_tender_recovery_host.sh`: actual production SQLite storage and orchestrator, actual cJSON, no source-string checks. Modes cover known final projection restart; all negative/unattempted; approval/decline/uncertain query; foreign identity; conflicting and concurrently changed purchase refs; payment/operation/member transaction faults; reversal reconciliation and uncertain restart; new compensation capability and intent/outcome faults; exact fiscal-proof acceptance/conflict; legacy CARD and terminal-owner blocks; stale CAS and missing snapshot table.

`tests/run_saga_storage_host.sh` retains the original 27 execution fault cases, updating only recovery expectations for the newly supported known results. Baseline old actual orchestrator fails the new `compensated` executable regression. These tests prove software semantics, not vendor query correctness or hardware behavior.
