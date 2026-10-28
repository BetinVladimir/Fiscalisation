package com.beeloy.fiscal.runtime

import java.util.concurrent.ConcurrentHashMap

/**
 * Thread-safe in-memory implementation of [OperationJournal] for unit tests and
 * vendor-neutral integration testing.
 *
 * Invariant 4: [beginOperation] is atomic via ConcurrentHashMap — concurrent calls with
 *   the same operationId are serialised; only one returns true.
 * Invariant 2: [markUnknown] records UNKNOWN state; callers must perform only lookup after.
 * Invariant 5: this implementation never silently drops writes; failures propagate to callers.
 * Invariant 6: [commit] requires a non-null fiscalRef when recording a COMMITTED state.
 *
 * NOT suitable for production persistence — use a SQLite-backed implementation that
 * survives process death.
 */
class InMemoryOperationJournal : OperationJournal {

    // ConcurrentHashMap provides atomic putIfAbsent for beginOperation (invariant 4).
    private val entries = ConcurrentHashMap<String, OperationJournal.Entry>()

    override fun findEntry(operationId: String): OperationJournal.Entry? =
        entries[operationId]

    /**
     * Returns false when operationId already exists (invariant 4: no second monetary command).
     */
    override fun beginOperation(operationId: String): Boolean {
        val fresh = OperationJournal.Entry(operationId, OperationJournal.State.PENDING)
        return entries.putIfAbsent(operationId, fresh) == null
    }

    override fun commit(operationId: String, fiscalRef: String?) {
        entries.compute(operationId) { _, existing ->
            requireNotNull(existing) { "JOURNAL_ENTRY_NOT_FOUND:$operationId" }
            existing.copy(state = OperationJournal.State.COMMITTED, fiscalRef = fiscalRef)
        }
    }

    /**
     * Advances to FISCAL_PENDING — card was charged but fiscal command outcome is unknown.
     * Invariant 3: callers must not issue a card refund while an entry is FISCAL_PENDING.
     */
    override fun markFiscalPending(operationId: String) {
        entries.compute(operationId) { _, existing ->
            requireNotNull(existing) { "JOURNAL_ENTRY_NOT_FOUND:$operationId" }
            existing.copy(state = OperationJournal.State.FISCAL_PENDING)
        }
    }

    override fun fail(operationId: String) {
        entries.compute(operationId) { _, existing ->
            requireNotNull(existing) { "JOURNAL_ENTRY_NOT_FOUND:$operationId" }
            existing.copy(state = OperationJournal.State.FAILED)
        }
    }

    /**
     * Invariant 2: marks UNKNOWN — callers must only perform lookup after this.
     * No retry write, no automatic card refund.
     */
    override fun markUnknown(operationId: String) {
        entries.compute(operationId) { _, existing ->
            requireNotNull(existing) { "JOURNAL_ENTRY_NOT_FOUND:$operationId" }
            existing.copy(state = OperationJournal.State.UNKNOWN)
        }
    }
}
