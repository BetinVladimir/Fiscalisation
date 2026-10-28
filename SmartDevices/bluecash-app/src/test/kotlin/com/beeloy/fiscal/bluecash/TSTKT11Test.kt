package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.time.Instant

/**
 * TST-KT-11: Journal ACK hash verification (BUG-030).
 *
 * G: events seq 1…5.
 * W: acknowledge(5, hashOfOtherEvent).
 * T: JournalAckMismatch; checkpoint stays 0; purgeBefore deletes nothing.
 *    Correct hash → checkpoint advances to 5.
 *    Chain break at seq 3 → rejection.
 */
class TSTKT11Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    private fun journal() = SignedMemoryJournal(signer())

    // G: events seq 1..5; W: acknowledge(5, wrongHash); T: rejected, checkpoint stays 0
    @Test fun acknowledgeWithWrongHashRejected() {
        val j = journal()
        val events = (1..5).map { j.append("op-11-$it", "ACCEPTED", "state=ACCEPTED&seq=$it") }
        val wrongHash = "0".repeat(64)

        try {
            j.acknowledge(events.last().sequence, wrongHash, "ack-11-bad")
            fail("acknowledge with wrong hash must throw")
        } catch (e: IllegalArgumentException) {
            assertTrue("error must mention HASH or ACK",
                e.message?.let {
                    it.contains("HASH", ignoreCase = true) || it.contains("ACK", ignoreCase = true)
                } == true)
        }
        val (seq, _) = j.checkpoint()
        assertEquals("checkpoint must stay at 0 after hash mismatch", 0L, seq)
    }

    // purgeBefore deletes nothing when checkpoint is 0
    @Test fun purgeBeforeDeletesNothingWhenCheckpointIsZero() {
        val j = journal()
        (1..5).forEach { j.append("op-11-p$it", "ACCEPTED", "state=ACCEPTED") }
        val deleted = j.purgeBefore(Instant.parse("2099-01-01T00:00:00Z"))
        assertEquals("purgeBefore must delete 0 events when checkpoint=0", 0, deleted)
    }

    // Correct hash → checkpoint advances to 5
    @Test fun correctHashAdvancesCheckpoint() {
        val j = journal()
        val events = (1..5).map { j.append("op-11-c$it", "ACCEPTED", "state=ACCEPTED") }
        val last = events.last()

        j.acknowledge(last.sequence, last.hash, "ack-11-ok")
        val (seq, _) = j.checkpoint()
        assertEquals("checkpoint must advance to last sequence", last.sequence, seq)
    }

    // Single event: correct hash → checkpoint = 1
    @Test fun singleEventCorrectHash() {
        val j = journal()
        val event = j.append("op-11-s1", "ACCEPTED", "state=ACCEPTED")
        j.acknowledge(event.sequence, event.hash, "ack-11-s1")
        val (seq, _) = j.checkpoint()
        assertEquals(event.sequence, seq)
    }

    // Wrong event hash (hash from different event) → rejected
    @Test fun wrongHashFromDifferentEventRejected() {
        val j = journal()
        val e1 = j.append("op-11-e1", "ACCEPTED", "state=ACCEPTED&seq=1")
        val e2 = j.append("op-11-e2", "ACCEPTED", "state=ACCEPTED&seq=2")
        // Use hash of e1 to acknowledge e2 — must fail
        try {
            j.acknowledge(e2.sequence, e1.hash, "ack-11-cross")
            fail("hash from different event must be rejected")
        } catch (e: IllegalArgumentException) {
            // expected
        }
        val (seq, _) = j.checkpoint()
        assertEquals("checkpoint must not advance on cross-hash", 0L, seq)
    }
}
