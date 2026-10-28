package com.beeloy.fiscal.runtime

import org.junit.Assert.*
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicInteger

/**
 * Unit tests for InMemoryOperationJournal.
 *
 * Invariant 2: UNKNOWN ≠ FAILED; no retry or auto-refund after markUnknown.
 * Invariant 4: Same key → no second document (beginOperation returns false on dup).
 * Invariant 5: Storage errors propagate (operations on missing entries throw).
 */
class InMemoryOperationJournalTest {

    // ── beginOperation / findEntry ────────────────────────────────────────────

    @Test fun beginOperationReturnsTrueForNewId() {
        val journal = InMemoryOperationJournal()
        assertTrue("First beginOperation must return true", journal.beginOperation("op-01"))
    }

    @Test fun beginOperationReturnsFalseForDuplicateId() {
        val journal = InMemoryOperationJournal()
        journal.beginOperation("op-02")
        assertFalse(
            "Second beginOperation with same id must return false (invariant 4)",
            journal.beginOperation("op-02")
        )
    }

    @Test fun findEntryReturnsNullForUnknownId() {
        val journal = InMemoryOperationJournal()
        assertNull("findEntry for unknown id must return null", journal.findEntry("no-such"))
    }

    @Test fun findEntryReturnsPendingAfterBegin() {
        val journal = InMemoryOperationJournal()
        journal.beginOperation("op-03")
        val entry = journal.findEntry("op-03")
        assertNotNull("entry must exist after beginOperation", entry)
        assertEquals("state must be PENDING", OperationJournal.State.PENDING, entry!!.state)
        assertNull("fiscalRef must be null before commit", entry.fiscalRef)
    }

    // ── commit ────────────────────────────────────────────────────────────────

    @Test fun commitAdvancesToCommitted() {
        val journal = InMemoryOperationJournal()
        journal.beginOperation("op-04")
        journal.commit("op-04", "BON-0001")
        val entry = journal.findEntry("op-04")!!
        assertEquals("state must be COMMITTED", OperationJournal.State.COMMITTED, entry.state)
        assertEquals("fiscalRef must be set from FU response (invariant 6)", "BON-0001", entry.fiscalRef)
    }

    @Test fun commitWithNullFiscalRefRecordsNull() {
        val journal = InMemoryOperationJournal()
        journal.beginOperation("op-05")
        journal.commit("op-05", null)
        val entry = journal.findEntry("op-05")!!
        assertEquals(OperationJournal.State.COMMITTED, entry.state)
        assertNull("null fiscalRef stored as null", entry.fiscalRef)
    }

    // ── markFiscalPending (invariant 3) ───────────────────────────────────────

    @Test fun markFiscalPendingAdvancesToFiscalPending() {
        val journal = InMemoryOperationJournal()
        journal.beginOperation("op-06")
        journal.markFiscalPending("op-06")
        val entry = journal.findEntry("op-06")!!
        assertEquals(
            "state must be FISCAL_PENDING (invariant 3: no card refund allowed)",
            OperationJournal.State.FISCAL_PENDING, entry.state
        )
    }

    // ── fail ─────────────────────────────────────────────────────────────────

    @Test fun failAdvancesToFailed() {
        val journal = InMemoryOperationJournal()
        journal.beginOperation("op-07")
        journal.fail("op-07")
        assertEquals(OperationJournal.State.FAILED, journal.findEntry("op-07")!!.state)
    }

    // ── markUnknown (invariant 2) ─────────────────────────────────────────────

    @Test fun markUnknownAdvancesToUnknown() {
        val journal = InMemoryOperationJournal()
        journal.beginOperation("op-08")
        journal.markUnknown("op-08")
        val entry = journal.findEntry("op-08")!!
        assertEquals(
            "state must be UNKNOWN after timeout/lost-connection (invariant 2)",
            OperationJournal.State.UNKNOWN, entry.state
        )
    }

    @Test fun unknownIsDistinctFromFailed() {
        // Invariant 2: UNKNOWN ≠ FAILED — different code paths for recovery
        assertNotEquals(
            "UNKNOWN must not equal FAILED (invariant 2)",
            OperationJournal.State.UNKNOWN,
            OperationJournal.State.FAILED
        )
    }

    // ── Concurrent beginOperation race (invariant 4) ──────────────────────────

    @Test fun concurrentBeginOperationOnlyOneSucceeds() {
        val journal = InMemoryOperationJournal()
        val wins = AtomicInteger(0)
        val threads = 16
        val latch = CountDownLatch(1)
        val pool = Executors.newFixedThreadPool(threads)
        val futures = (1..threads).map {
            pool.submit<Boolean> {
                latch.await()
                journal.beginOperation("race-01")
            }
        }
        latch.countDown()
        futures.forEach { if (it.get()) wins.incrementAndGet() }
        pool.shutdown()
        assertEquals(
            "Only one thread must win beginOperation for the same key (invariant 4)",
            1, wins.get()
        )
    }

    // ── Operations on missing entries propagate errors (invariant 5) ──────────

    @Test fun commitOnMissingEntryThrows() {
        val journal = InMemoryOperationJournal()
        try {
            journal.commit("no-such", "BON-X")
            fail("commit on missing entry must throw (invariant 5)")
        } catch (e: IllegalArgumentException) {
            assertTrue("Error must mention operation id", e.message?.contains("no-such") == true)
        }
    }

    @Test fun markUnknownOnMissingEntryThrows() {
        val journal = InMemoryOperationJournal()
        try {
            journal.markUnknown("no-such")
            fail("markUnknown on missing entry must throw (invariant 5)")
        } catch (e: IllegalArgumentException) {
            // expected
        }
    }
}
