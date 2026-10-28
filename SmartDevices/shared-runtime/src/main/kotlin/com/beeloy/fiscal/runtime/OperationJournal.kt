package com.beeloy.fiscal.runtime

/**
 * Vendor-neutral durable operation journal.
 *
 * Invariant 4: same operationId → no second document. beginOperation returns false if already seen.
 * Invariant 2: markUnknown records UNKNOWN state; no retry, no automatic refund.
 * Invariant 5: storage errors propagate — callers must treat a storage error as a halt signal.
 * Invariant 6: fiscalRef must come only from the FU response, never synthesised.
 */
interface OperationJournal {

    enum class State { PENDING, COMMITTED, FISCAL_PENDING, FAILED, UNKNOWN }

    data class Entry(
        val operationId: String,
        val state: State,
        /** Fiscal reference from the FU response; null until committed. */
        val fiscalRef: String? = null,
    )

    /** Returns existing entry if operationId was already recorded. */
    fun findEntry(operationId: String): Entry?

    /**
     * Atomically records a new PENDING entry.
     * Returns false (no-op) if operationId already exists — callers must not proceed with
     * a second monetary command when false is returned (invariant 4).
     */
    fun beginOperation(operationId: String): Boolean

    /** Advances entry to COMMITTED with the FU-supplied fiscal reference. */
    fun commit(operationId: String, fiscalRef: String?)

    /**
     * Advances entry to FISCAL_PENDING: card was charged but fiscal failed.
     * Invariant 3: no refund until confirmed storno — callers must not issue a card refund
     * when an entry is in FISCAL_PENDING state.
     */
    fun markFiscalPending(operationId: String)

    /** Advances entry to FAILED. */
    fun fail(operationId: String)

    /**
     * Advances entry to UNKNOWN (write timed out or connection lost after send).
     * Invariant 2: callers must only perform lookup after this — no retry write, no refund.
     */
    fun markUnknown(operationId: String)
}
