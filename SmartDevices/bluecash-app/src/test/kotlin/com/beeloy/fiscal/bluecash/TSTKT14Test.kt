package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.security.MessageDigest

/**
 * TST-KT-14: Journal chain break detection (BUG-033).
 *
 * G: 10 journal events form a valid SHA-256 hash chain.
 * W: replace the payload of event 4 (keeping its stored hash unchanged).
 * T: verifyChain() returns an error at seq 4 (and only seq 4, not earlier);
 *    the unmodified chain returns no error.
 */
class TSTKT14Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    /**
     * Re-derive each event's hash from its fields and check it against the stored hash.
     * Also checks that each event's previousHash matches the preceding event's hash.
     * Returns the sequence of the first broken event, or null if the chain is intact.
     */
    private fun verifyChain(records: List<JournalRecord>): Long? {
        for (i in records.indices) {
            val r = records[i]
            val expectedPrev = if (i == 0) null else records[i - 1].hash
            // Canonical string matches SignedMemoryJournal.append():
            // "$seq\n$operationId\n$type\n$at\n${previous?:""}\n$payload"
            val canonical =
                "${r.sequence}\n${r.operationId}\n${r.type}\n${r.occurredAt}\n${expectedPrev ?: ""}\n${r.payload}"
            val recomputed =
                MessageDigest.getInstance("SHA-256")
                    .digest(canonical.toByteArray())
                    .joinToString("") { "%02x".format(it) }
            if (recomputed != r.hash) return r.sequence
            if (r.previousHash != expectedPrev) return r.sequence
        }
        return null
    }

    // Unmodified chain of 10 events must verify without error
    @Test fun unmodifiedChainIsValid() {
        val journal = SignedMemoryJournal(signer())
        val records = (1..10).map { i ->
            journal.append("chain-14-$i", "ACCEPTED", "seq=$i&data=payload$i")
        }
        assertEquals("intact chain must return null (no break)", null, verifyChain(records))
    }

    // Tampering payload of event 4 (index 3) while keeping its stored hash → break detected at seq 4
    @Test fun journalChainBreakDetected() {
        val journal = SignedMemoryJournal(signer())
        val records = (1..10).map { i ->
            journal.append("chain-14-$i", "ACCEPTED", "seq=$i&data=payload$i")
        }

        // Tamper: copy event at index 3 (seq=4) with a different payload, old hash preserved
        val tamperedRecords = records.toMutableList()
        tamperedRecords[3] = tamperedRecords[3].copy(payload = "TAMPERED_PAYLOAD_CHAIN_14")

        val breakAt = verifyChain(tamperedRecords)
        assertNotNull("tampered chain must report a break", breakAt)
        assertEquals("chain break must be reported at seq 4 (index 3)", 4L, breakAt)
    }

    // Events 1-3 before the tampered event must still pass verification
    @Test fun breakOnlyFromTamperedSeq() {
        val journal = SignedMemoryJournal(signer())
        val records = (1..10).map { i ->
            journal.append("chain-14b-$i", "ACCEPTED", "seq=$i&data=payload$i")
        }
        // Verify prefix (events 1-3) is intact — should return null (no break in first 3)
        val prefix = records.subList(0, 3)
        assertEquals("events 1-3 must be intact", null, verifyChain(prefix))
    }

    // Tamper event at index 0 (seq=1) → break detected at seq 1
    @Test fun chainBreakAtFirstEvent() {
        val journal = SignedMemoryJournal(signer())
        val records = (1..5).map { i ->
            journal.append("chain-14c-$i", "ACCEPTED", "seq=$i")
        }
        val tampered = records.toMutableList()
        tampered[0] = tampered[0].copy(payload = "TAMPERED_FIRST")
        val breakAt = verifyChain(tampered)
        assertEquals("break at first event must be seq 1", 1L, breakAt)
    }

    // Single-event chain: tamper its payload → break at seq 1
    @Test fun singleEventTamperedChainBreaks() {
        val journal = SignedMemoryJournal(signer())
        val record = journal.append("chain-14d-1", "ACCEPTED", "original-payload")
        val tampered = listOf(record.copy(payload = "bad-payload"))
        assertNotNull("tampered single-event chain must report break", verifyChain(tampered))
    }

    // Single-event chain intact → no break
    @Test fun singleEventIntactChainIsValid() {
        val journal = SignedMemoryJournal(signer())
        val record = journal.append("chain-14e-1", "ACCEPTED", "original-payload")
        assertNull("intact single-event chain must return null", verifyChain(listOf(record)))
    }
}
