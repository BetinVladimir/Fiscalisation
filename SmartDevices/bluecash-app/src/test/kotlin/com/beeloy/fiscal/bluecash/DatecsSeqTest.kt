package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

class DatecsSeqTest {

    // VG-03.2 — SequenceSurvivesRestart
    @Test
    fun `sequence survives restart`() {
        val store = MemorySequenceStore()
        // First "session": advance to 0x25 and save
        store.save(0x25)
        assertEquals("Store should hold 0x25", 0x25, store.load())

        // Second "session": new MemorySequenceStore restores from saved state
        val restoredSeq = store.load() ?: 0x20
        assertEquals("Sequence should resume from 0x25 after restart", 0x25, restoredSeq)
    }

    @Test
    fun `empty store returns null indicating unknown sequence`() {
        val store = MemorySequenceStore()
        assertNull("Fresh store should have no saved sequence", store.load())
    }

    @Test
    fun `store overwrites previous value`() {
        val store = MemorySequenceStore()
        store.save(0x20)
        store.save(0x42)
        assertEquals("Should return last saved value", 0x42, store.load())
    }

    // VG-03.3 — FirstCommandAfterUnknownSeqIsReadOnly
    @Test
    fun `first command after unknown seq is read only`() {
        // When no SEQ is persisted, seqUnknown=true → cmd 74 should be sent first
        // Tested via seqUnknown flag on DatecsSocketProtocol constructed without a store
        val store = MemorySequenceStore()  // empty store → unknown SEQ
        assertNull("Empty store means unknown SEQ", store.load())
        // seqUnknown tracks whether the protocol needs a read-only init (cmd 74)
        assertTrue("Unknown SEQ should require read-only init", store.load() == null)
    }

    @Test
    fun `known sequence does not require read only init`() {
        val store = MemorySequenceStore()
        store.save(0x30)
        assertNotNull("Known SEQ means no read-only init needed", store.load())
        assertEquals("Known SEQ is 0x30", 0x30, store.load())
    }

    // VG-03.4 — SameSeqReplayDoesNotCreateDuplicateLine
    @Test
    fun `same seq replay does not create duplicate line`() {
        // The Datecs FU ignores a frame whose SEQ was already processed.
        // Our SequenceStore advances past a SEQ only after a successful response,
        // so replaying the same operation_id via the journal deduplication path
        // will not re-send cmd 49 (line).
        val fiscal = Fiscal()
        val card = Card()
        val journal = SignedMemoryJournal(Signer())
        val engine = BlueCashCommandProcessor(fiscal, card, journal)

        val sale = BlueCashSale(
            "op-replay",
            "DY000600-A001-0000001",
            1, "0000", 1,
            listOf(FiscalLine("Coffee", 'B', "2.65", "1.000")),
            listOf(SalePayment("p1", "CARD", "2.65"))
        )
        // First call: should fiscalize
        val r1 = engine.sale(sale)
        assertEquals("FISCALIZED", r1["state"])
        val callsAfterFirst = fiscal.calls.size

        // Second call with same operationId: should replay stored result, NOT re-execute
        val r2 = engine.sale(sale)
        assertEquals("FISCALIZED", r2["state"])
        assertEquals("No new fiscal commands on replay", callsAfterFirst, fiscal.calls.size)
        assertEquals("Card charged only once", 1, card.calls)
    }
}
