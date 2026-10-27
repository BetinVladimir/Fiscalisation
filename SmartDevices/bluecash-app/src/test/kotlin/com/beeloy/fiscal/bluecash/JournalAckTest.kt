package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.time.Instant

/** BUG-030: acknowledge() must verify hash against stored event_hash, not just format. */
class JournalAckTest {

  private fun signer() = object : TransactionSigner {
    override val keyId = "test"
    override fun sign(hash: ByteArray) = hash
  }

  private fun journal() = SignedMemoryJournal(signer())

  // BUG-030.2: wrong hash is rejected; checkpoint does not advance.
  @Test fun `acknowledgeWithWrongHashRejected`() {
    val j = journal()
    val rec = j.append("op-1", "ACCEPTED", "state=ACCEPTED")
    val wrongHash = "0".repeat(64)
    try {
      j.acknowledge(rec.sequence, wrongHash, "ack-1")
      fail("acknowledge with wrong hash must throw")
    } catch (e: IllegalArgumentException) {
      assertTrue("message must mention hash or mismatch",
        e.message?.contains("HASH", ignoreCase = true) == true ||
        e.message?.contains("ACK", ignoreCase = true) == true)
    }
  }

  @Test fun `acknowledgeDoesNotAdvanceCheckpointOnMismatch`() {
    val j = journal()
    val rec = j.append("op-1", "ACCEPTED", "state=ACCEPTED")
    val wrongHash = "a".repeat(64)
    try { j.acknowledge(rec.sequence, wrongHash, "ack-1") } catch (_: Exception) {}
    val (seq, _) = j.checkpoint()
    assertEquals("checkpoint must not advance on hash mismatch", 0L, seq)
  }

  // BUG-030.4: purgeBefore must never delete unacknowledged rows.
  @Test fun `purgeBeforeNeverDeletesUnacknowledged`() {
    val past = Instant.parse("2020-01-01T00:00:00Z")
    val j = SignedMemoryJournal(signer(), clock = { past })
    j.append("op-1", "ACCEPTED", "state=ACCEPTED")
    // Attempt purge with a far-future cutoff — nothing acknowledged yet.
    val deleted = j.purgeBefore(Instant.parse("2099-01-01T00:00:00Z"))
    assertEquals("unacknowledged events must never be purged", 0, deleted)
    assertEquals("event must still be findable", 1, j.find("op-1").size)
  }

  // Positive case: correct hash is accepted.
  @Test fun `acknowledgeWithCorrectHashAdvancesCheckpoint`() {
    val j = journal()
    val rec = j.append("op-1", "ACCEPTED", "state=ACCEPTED")
    j.acknowledge(rec.sequence, rec.hash, "ack-1")
    val (seq, _) = j.checkpoint()
    assertEquals(rec.sequence, seq)
  }
}
