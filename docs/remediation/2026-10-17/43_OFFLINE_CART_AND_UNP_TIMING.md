# Fresh offline cart creation and regulatory identity timing

Status: OPEN. The accepted source-first boundary remains in [ADR-SUPTO-001](../../decisions/ADR-SUPTO-001-thin-pos.md). It moves commercial cart ownership to MiniPOS and retires direct native first-tap root API calls; it does not waive fiscal identity/readiness requirements. The repository's [Annex29 baseline](../../SUPTO/Приложение%20№%2029.md), points8/9, addresses readiness and UNP at sale-information entry. The latest full normative text still needs independent verification; this document does not establish current legal compliance.

## Observed software gaps

1. `App.add` always calls source `/orders` and `/orders/{id}/lines`. A completely unavailable source backend prevents a new cart; a cached execution grant is considered only during checkout of an existing cart. Current offline checkout tests do not establish fresh offline sale cycles.
2. The source order projection has no regulatory UNP, and trusted root/S3 preparation occurs at payment. The old matrix's direct `/sales:open-with-line` anchor is obsolete, but replacing that anchor cannot close readiness/identity timing. This remains UI-MP-005; fresh offline creation is UI-MP-027.
3. Successful local fiscal execution must retain durable result/receipt evidence until authoritative synchronization. Clearing the only checkout journal immediately is not proof of later cloud/source materialization. Cloud final-outcome repairs do not close this offline requirement.

## Required implementation sequence

- Define the regulated sale-entry boundary in the approved profile and represent it in source/S3 contracts. Preserve source order IDs, local commercial surrogates and real root sale IDs as distinct identities. Do not invent a server-owned order/sale ID or generate a regulatory UNP in the POS.
- In the online path, let the trusted source/backend atomically associate first-line sale entry with root-owned readiness/identity reservation; synchronize further commercial edits under versions. Keep the thin POS boundary and avoid a second independent sale-opening request from the UI.
- In the offline path, persist the commercial draft before I/O and request readiness/identity from S3 using bounded signed authority and the authenticated device channel. S3 alone reserves a leased counter durably. A lease, valid identity/time/binding and device availability are prerequisites; missing evidence fails closed. Cancellation/abandonment must be auditable and cannot recycle a counter.
- Persist the complete authorized operation/receipt/UNP/result and synchronization checkpoint independently of the currently visible cart. After local completion a later sale may proceed only under the reviewed local policy, while the completed record remains in the durable synchronization queue.
- Materialize cloud/source state idempotently from authenticated S3 evidence after reconnection, preserving original IDs, sequence, payload digest and commercial/fiscal linkage. Existing unknown results remain quarantined; recovery never repeats a purchase/fiscalization to discover the result.

## Tests to add before closing

- Real source/root/S3 tests: first-line transaction and identity linkage, second-line edits, rejected first-line rollback, readiness missing/stale and operator/device/binding mismatch. No POS-generated or reassigned UNP.
- Actual S3 SQLite tests: concurrent reservations, restart after every checkpoint, exhausted ranges, cancelled/abandoned drafts, multiple completed unsynchronized sales and monotonic counters.
- Android/iOS/web native/App tests: start a new cart with source HTTP unavailable and a valid authenticated S3 route; barcode/manual lines, CASH/CARD/split, restart and reconnect. Missing authority or expired device session must block without a physical command.
- Real synchronization tests: duplicate/out-of-order acknowledgments, lost source response, foreign tenant/session, payload/reference mismatch, partial receipt import and repeated restart. Exactly one source/root sale association, purchase, fiscal receipt and journal completion.

Physical devices, offline revocation/equivalence and the applicable external НАП review remain separate acceptance gates. ESP32-S3 keys remain internal NVS; ATECC608 is unused.

## Primary-source follow-up2026-10-21

[Review49](49_NAP_PRIMARY_REQUIREMENTS_RECHECK.md) verifies the available official consolidated candidate, Annex29 points8/9.1 on printed pages279–280. The timing/readiness gap remains OPEN; source-first architecture does not establish an exception. Full legislative currency at the release date remains a separate unresolved gate.
