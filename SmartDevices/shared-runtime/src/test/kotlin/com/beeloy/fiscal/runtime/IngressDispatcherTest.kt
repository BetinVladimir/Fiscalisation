package com.beeloy.fiscal.runtime

import org.junit.Assert.*
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger

/**
 * SMS-02.5: IngressDispatcher tests.
 *
 * Invariant 4: two transports with the same operationId → one execute call.
 * Invariant 2: UNKNOWN after execute failure → no retry, no auto refund.
 * Restart recovery: PENDING entry from previous run returns RECOVERY_REQUIRED.
 */
class IngressDispatcherTest {

    private fun freshJournal() = InMemoryOperationJournal()

    private fun alwaysSucceed(fiscalRef: String = "BON-001"): (String, Map<String, String>) -> Map<String, String> =
        { _, _ -> mapOf("state" to "COMMITTED", "fiscal_reference" to fiscalRef) }

    // ── Single dispatch succeeds ──────────────────────────────────────────────

    @Test fun dispatchNewOperationReturnsCommitted() {
        val dispatcher = IngressDispatcher(freshJournal(), alwaysSucceed("BON-100"))
        val result = dispatcher.dispatch("op-01", emptyMap())
        assertEquals("state", "COMMITTED", result["state"])
    }

    @Test fun dispatchRecordsJournalCommitted() {
        val journal = freshJournal()
        val dispatcher = IngressDispatcher(journal, alwaysSucceed("BON-200"))
        dispatcher.dispatch("op-02", emptyMap())
        val entry = journal.findEntry("op-02")
        assertNotNull("entry must be journalled", entry)
        assertEquals("must be COMMITTED", OperationJournal.State.COMMITTED, entry!!.state)
        assertEquals("fiscalRef from execute result (invariant 6)", "BON-200", entry.fiscalRef)
    }

    // ── Duplicate operationId from same or second transport ───────────────────

    @Test fun duplicateOperationIdReturnsCachedResult() {
        val executeCount = AtomicInteger(0)
        val execute: (String, Map<String, String>) -> Map<String, String> = { _, _ ->
            executeCount.incrementAndGet()
            mapOf("state" to "COMMITTED", "fiscal_reference" to "BON-300")
        }
        val dispatcher = IngressDispatcher(freshJournal(), execute)

        val result1 = dispatcher.dispatch("op-dup", emptyMap())
        val result2 = dispatcher.dispatch("op-dup", emptyMap())

        assertEquals("execute must be called exactly once (invariant 4)", 1, executeCount.get())
        assertEquals("first result state", "COMMITTED", result1["state"])
        assertEquals("second result must reflect COMMITTED cache", "COMMITTED", result2["state"])
    }

    @Test fun twoTransportsSequentiallyExecuteOnce() {
        // Scenario: MQTT delivers operationId then BLE delivers the same one slightly after.
        // The second delivery sees the journalled COMMITTED result — no second execute.
        val executeCount = AtomicInteger(0)
        val execute: (String, Map<String, String>) -> Map<String, String> = { _, _ ->
            executeCount.incrementAndGet()
            mapOf("state" to "COMMITTED", "fiscal_reference" to "BON-RACE")
        }
        val dispatcher = IngressDispatcher(freshJournal(), execute)

        // First transport (e.g. MQTT)
        val result1 = dispatcher.dispatch("op-race", emptyMap())
        // Second transport (e.g. BLE) delivers the same operationId
        val result2 = dispatcher.dispatch("op-race", emptyMap())

        assertEquals("execute must be called exactly once (invariant 4)", 1, executeCount.get())
        assertEquals("first result state", "COMMITTED", result1["state"])
        assertEquals("second transport must see COMMITTED cache", "COMMITTED", result2["state"])
    }

    // ── Restart recovery: PENDING entry survives ───────────────────────────────

    @Test fun pendingEntryFromPreviousRunReturnsRecoveryRequired() {
        // Simulate: process died between beginOperation and execute
        val journal = freshJournal()
        journal.beginOperation("op-crashed")  // left in PENDING state by crashed process
        // New dispatcher (simulates restart) sees the dangling PENDING
        val executeCount = AtomicInteger(0)
        val dispatcher = IngressDispatcher(journal) { _, _ ->
            executeCount.incrementAndGet()
            mapOf("state" to "COMMITTED", "fiscal_reference" to "BON-X")
        }
        val result = dispatcher.dispatch("op-crashed", emptyMap())
        assertEquals("must return UNKNOWN/RECOVERY_REQUIRED for dangling PENDING (invariant 2)",
            "UNKNOWN", result["state"])
        assertEquals(
            "must not re-execute (invariant 2: UNKNOWN ≠ FAILED, no retry)",
            0, executeCount.get()
        )
    }

    // ── Execute failure → UNKNOWN (invariant 2) ───────────────────────────────

    @Test fun executeFailureRecordsUnknownNeverFailed() {
        val journal = freshJournal()
        val dispatcher = IngressDispatcher(journal) { _, _ ->
            throw RuntimeException("DEVICE_TIMEOUT")
        }
        val result = dispatcher.dispatch("op-timeout", emptyMap())
        assertEquals("must be UNKNOWN on exception (invariant 2)", "UNKNOWN", result["state"])
        val entry = journal.findEntry("op-timeout")
        assertNotNull("entry must exist after failure", entry)
        assertEquals("journal entry must be UNKNOWN (invariant 2)",
            OperationJournal.State.UNKNOWN, entry!!.state)
        assertNotEquals("UNKNOWN must not become FAILED (invariant 2)",
            OperationJournal.State.FAILED, entry.state)
    }
}
