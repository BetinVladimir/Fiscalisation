# Payment saga: durable intent, partial CARD failure and blocked reconciliation

This package closes ignored payment checkpoint errors and unsafe partial-CARD rejection. It does not close GAP019/020 end to end: automatic per-tender reconciliation remains required. GAP021 synthetic fiscal reference remains open.

## Durable contract

Every CARD purchase requires a real `DurableStorage` journal. `payment_saga` retains an operation phase independently of operation result projection; `payment_saga_members` freezes the CARD payment membership. Beginning a saga requires the same operation/receipt/amount, RESERVED tenders, EXECUTING operation, unique IDs, and no previous saga.

Each checkpoint atomically updates the payment row, saga phase and operation state under one SQLite transaction and shared connection mutex. CAS verifies operation, receipt, amount, expected phase/state, membership and a nonterminal owning operation. It cannot revive a committed operation or transition an unrelated CASH payment. Original purchase terminal reference, RRN and auth remain intact during compensation. Separate reversal outcome reference/RRN/auth are stored in the saga member row in the same transaction as its outcome state.

Purchase phases: RESERVED → CARD_SENDING before the call; CARD_SENDING → CARD_APPROVED/CARD_DECLINED/CARD_NOT_SENT/CARD_UNKNOWN afterward. Only once every frozen CARD member is durably approved may PAYMENTS → FISCAL_SENDING occur before fiscal IO.

A failed checkpoint stops the sequence immediately. Failure before a side effect sends nothing. Failure after a side effect leaves its durable SENDING intent and returns UNKNOWN with available outcome references. If storage cannot persist the returned outcome, success is never inferred from the earlier intent; the operation requires reconciliation. No fallback vendor call is attempted while persistence is failing.

## Partial failure and compensation

A first proven decline/not-sent can return REJECTED. A later proven decline/not-sent compensates prior approved tenders in reverse order only when the IO declares an independently verified reversal profile and the original RRN exists. The reverse API accepts original **RRN**, never STAN or a new payment UUID.

Each reversal checkpoints CARD_APPROVED → REVERSAL_SENDING before vendor IO, then REVERSED/REVERSAL_UNKNOWN/REVERSAL_FAILED afterward. Only all durably resolved members permit the final COMPENSATED phase. Missing capability/RRN persists COMPENSATION_REQUIRED and returns UNKNOWN without issuing a guessed reversal. Any uncertain or failed reversal stops subsequent calls and returns UNKNOWN. An uncertain current purchase does not automatically compensate earlier approvals; current uncertainty must first be reconciled.

The physical `EspIdfDeviceIo` inherits the default **unverified** reversal capability. Software test IO explicitly declares its simulated verified contract. No vendor BORICA reversal profile has been established by these tests. Actual physical compensation remains blocked pending vendor protocol evidence, reference mapping, request/response vectors and HIL.

CARD SALE_REVERSE currently has no immutable original payment RRN in its request/lookup contract. It is rejected before money IO with CARD_REFUND_REFERENCE_REQUIRED rather than issuing VOID_PURCHASE using the new tender UUID. CASH fiscal storno remains supported. Adding an immutable original payment reference and the appropriate vendor refund-versus-void semantics is required to restore supported physical CARD reversal.

## Restart and required next package

Any existing new payment saga blocks executor reentry from purchasing, reversing or fiscalizing again. The marker survives an operation-level RECOVERY_REQUIRED result and process restart. `ProfileExecutor::recover` returns PAYMENT_SAGA_RECONCILIATION_REQUIRED without calling the current single-payment device recovery path for these operations. Existing legacy/cash recovery remains unchanged.

This is a safety invariant, not completed recovery. The next owner-scoped package must load **all** frozen payment members and their individual states/references, prove physical fiscal receipt presence/absence for FISCAL_SENDING, reconcile CARD_SENDING/CARD_UNKNOWN and reversal uncertainty with vendor-supported lookup, and atomically project the final receipt/payment/outbox result. It must never resend a purchase or reversal merely because a response/checkpoint is missing. Operation-level aggregate references must not replace per-tender evidence. Physical receipt proof depends on the separate GAP021 work.

## Executable evidence

`tests/run_saga_storage_host.sh` compiles actual production DurableStorage and ProfileExecutor against real SQLite/cJSON/OpenSSL. Test IO replaces only final-device calls, with explicit approved/declined/unknown results and capability declaration. Fault triggers target each transaction checkpoint: begin table/member/operation; purchase intent row/operation; purchase outcome row/operation; pre-fiscal phase/operation; reversal intent row/operation; reversal outcome row/operation; final compensation phase/operation. Every scenario reopens SQLite and verifies no duplicate vendor side effect after reentry/recovery.

Additional scenarios cover two approved tenders, later decline/not-sent, later unknown without automatic reversal, unverified reversal profile, uncertain reversal, fiscal unknown, provably rejected fiscal call, missing original refund reference, CAS foreign ownership/receipt/amount/membership, duplicate initial IDs and terminal-operation protection. Existing money vendor-byte tests and native profile tests retain their behavior using complete durable fixtures; each money test invocation resets its fixture as an independent operation.
