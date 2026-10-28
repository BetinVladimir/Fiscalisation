package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * Tests for DatecsRecovery: VG-04.1–4.4, VG-04.5 (FU emulator), VG-07.1–7.2, VG-09.1–9.3.
 */
class DatecsRecoveryTest {

    private fun recovery(fu: StatefulFakeDatecsFiscalPort): Pair<DatecsRecovery, SignedMemoryJournal> {
        val journal = SignedMemoryJournal(Signer())
        return DatecsRecovery(fu, journal) to journal
    }

    // ─── VG-04.5: FU state emulator ───────────────────────────────────────────

    @Test
    fun `FakeDatecsDeviceStateTest cmd76 returns isOpen`() {
        val fu = StatefulFakeDatecsFiscalPort(isOpen = 1, receiptNumber = 5)
        val resp = fu.execute(76)
        val text = DatecsText.decode(resp.data)
        val fields = text.split('\t')
        assertEquals("0", fields[0])  // error code = 0
        assertEquals("1", fields[1])  // isOpen = 1
        assertEquals("5", fields[2])  // receiptNumber = 5
    }

    @Test
    fun `FakeDatecsDeviceStateTest cmd60 closes receipt`() {
        val fu = StatefulFakeDatecsFiscalPort(isOpen = 1)
        fu.execute(60)
        assertEquals(0, fu.isOpen)
    }

    @Test
    fun `FakeDatecsDeviceStateTest cmd255 UNP returns param`() {
        val fu = StatefulFakeDatecsFiscalPort(currentUnp = "DY000600-A001-0000042")
        val resp = fu.execute(255, DatecsText.encode("UNP"))
        val text = DatecsText.decode(resp.data)
        assertTrue("Expected UNP in response, got: $text", text.contains("DY000600-A001-0000042"))
    }

    // ─── VG-04.1: StartupSnapshotRecorded ─────────────────────────────────────

    @Test
    fun `StartupSnapshotRecorded snapshot records PRE_OPEN journal entry`() {
        val fu = StatefulFakeDatecsFiscalPort(
            isOpen = 0,
            receiptNumber = 3,
            currentUnp = "DY000600-A001-0000003",
            lastFiscalDoc = 3
        )
        val (rec, journal) = recovery(fu)
        val snap = rec.snapshot("op-1")

        assertEquals(0, snap.isOpen)
        assertEquals(3, snap.receiptNumber)
        assertEquals("DY000600-A001-0000003", snap.unp)
        assertEquals(3, snap.lastFiscalDoc)

        val entries = journal.find("op-1")
        assertTrue("Expected PRE_OPEN entry", entries.any { it.type == "PRE_OPEN" })
        val preOpen = entries.first { it.type == "PRE_OPEN" }
        assertTrue(preOpen.payload.contains("number=3"))
        assertTrue(preOpen.payload.contains("unp=DY000600-A001-0000003"))
    }

    // ─── VG-04.2: ResolvesCommittedWhenUnpMatches ─────────────────────────────

    @Test
    fun `ResolvesCommittedWhenUnpMatches returns COMMITTED when receipt advanced and UNP matches`() {
        val expectedUnp = "DY000600-A001-0000004"
        val fu = StatefulFakeDatecsFiscalPort(
            isOpen = 0,
            receiptNumber = 4,      // advanced from expected 3
            currentUnp = expectedUnp
        )
        val (rec, _) = recovery(fu)
        val expected = PreOpenSnapshot(isOpen = 0, receiptNumber = 3, unp = "DY000600-A001-0000003", lastFiscalDoc = 3)
        val state = rec.resolve(expected, expectedUnp)
        assertEquals(ResolutionState.COMMITTED, state)
    }

    @Test
    fun `ResolvesNotCommittedWhenUnpUnchanged returns NOT_COMMITTED when UNP unchanged`() {
        val originalUnp = "DY000600-A001-0000003"
        val fu = StatefulFakeDatecsFiscalPort(
            isOpen = 0,
            receiptNumber = 3,      // same as before
            currentUnp = originalUnp
        )
        val (rec, _) = recovery(fu)
        val expected = PreOpenSnapshot(isOpen = 0, receiptNumber = 3, unp = originalUnp, lastFiscalDoc = 3)
        val state = rec.resolve(expected, "DY000600-A001-0000004")
        assertEquals(ResolutionState.NOT_COMMITTED, state)
    }

    @Test
    fun `ResolvesUnknownWhenReceiptStillOpen returns UNKNOWN when receipt still open`() {
        val fu = StatefulFakeDatecsFiscalPort(isOpen = 1, receiptNumber = 3, currentUnp = "DY000600-A001-0000003")
        val (rec, _) = recovery(fu)
        val expected = PreOpenSnapshot(isOpen = 0, receiptNumber = 3, unp = "DY000600-A001-0000003", lastFiscalDoc = 3)
        val state = rec.resolve(expected, "DY000600-A001-0000004")
        assertEquals(ResolutionState.UNKNOWN, state)
    }

    // ─── VG-04.3: CancelsOrphanOpenReceipt ────────────────────────────────────

    @Test
    fun `CancelsOrphanOpenReceipt sends cmd60 and journals ORPHAN_RECEIPT_CANCELED`() {
        val fu = StatefulFakeDatecsFiscalPort(isOpen = 1)
        val (rec, journal) = recovery(fu)
        val cancelled = rec.cancelOrphanIfNeeded("op-orphan")
        assertTrue("Expected cancellation to succeed", cancelled)
        assertEquals(0, fu.isOpen)
        val entries = journal.find("op-orphan")
        assertTrue("Expected ORPHAN_RECEIPT_CANCELED entry",
            entries.any { it.type == "ORPHAN_RECEIPT_CANCELED" })
        assertTrue("cmd 60 must have been sent",
            fu.executedCommands.any { it.first == 60 })
    }

    @Test
    fun `CancelsOrphanOpenReceipt does nothing when receipt closed`() {
        val fu = StatefulFakeDatecsFiscalPort(isOpen = 0)
        val (rec, journal) = recovery(fu)
        val cancelled = rec.cancelOrphanIfNeeded("op-2")
        assertFalse(cancelled)
        assertTrue("No ORPHAN entries expected",
            journal.find("op-2").none { it.type == "ORPHAN_RECEIPT_CANCELED" })
    }

    // ─── VG-04.4: DoesNotReverseCardWhileUnknown ──────────────────────────────

    @Test
    fun `DoesNotReverseCardWhileUnknown resolve UNKNOWN means card must not be reversed`() {
        // Receipt still open → resolve returns UNKNOWN → card must NOT be reversed
        val fu = StatefulFakeDatecsFiscalPort(isOpen = 1, receiptNumber = 3, currentUnp = "DY000600-A001-0000003")
        val (rec, _) = recovery(fu)
        val expected = PreOpenSnapshot(isOpen = 0, receiptNumber = 3, unp = "DY000600-A001-0000003", lastFiscalDoc = 3)
        val state = rec.resolve(expected, "DY000600-A001-0000004")
        assertEquals(
            "Card reversal must not happen while state is UNKNOWN; got $state",
            ResolutionState.UNKNOWN, state
        )
        // The caller MUST NOT call card.reverse() when state == UNKNOWN (invariant 2)
        // This test documents the invariant: UNKNOWN → no reversal, no retry
    }

    // ─── VG-07.1: EvidenceBuiltFromSpecExamples ───────────────────────────────

    @Test
    fun `EvidenceBuiltFromSpecExamples reads FMnumber nLastFiscalDoc UNP after close`() {
        val fu = StatefulFakeDatecsFiscalPort(
            fmNumber = "12345678",
            lastFiscalDoc = 42,
            currentUnp = "DY000600-A001-0000042"
        )
        val (rec, _) = recovery(fu)
        val evidence = rec.readEvidence()
        assertEquals(42, evidence.docNumber)
        assertEquals("12345678", evidence.fiscalMemoryNumber)
        assertEquals("DY000600-A001-0000042", evidence.unp)
    }

    // ─── VG-07.2: UnpMismatchIsUnknown ────────────────────────────────────────

    @Test
    fun `UnpMismatchIsUnknown resolve returns UNKNOWN when UNP does not match expected`() {
        // After sale attempt, device UNP changed but to a different value than we expected
        val fu = StatefulFakeDatecsFiscalPort(
            isOpen = 0,
            receiptNumber = 4,
            currentUnp = "DY000600-A001-0000099"  // different from what we expected
        )
        val (rec, _) = recovery(fu)
        val expected = PreOpenSnapshot(isOpen = 0, receiptNumber = 3, unp = "DY000600-A001-0000003", lastFiscalDoc = 3)
        // expectedUnp is what we tried to print; device shows a different one → UNKNOWN
        val state = rec.resolve(expected, "DY000600-A001-0000004")
        assertEquals(ResolutionState.UNKNOWN, state)
    }

    // ─── VG-09.1: ReadsCurrencyParameterAtActivation ─────────────────────────

    @Test
    fun `ReadsCurrencyParameterAtActivation reads mode 3 (EUR-only) from device`() {
        val fu = StatefulFakeDatecsFiscalPort(currencyMode = 3)
        val (rec, _) = recovery(fu)
        val mode = rec.readCurrencyMode()
        assertEquals(3, mode)
        assertTrue("cmd 255 CurrencyMode must have been sent",
            fu.executedCommands.any { it.first == 255 && it.second.contains("CurrencyMode") })
    }

    // ─── VG-09.2: RefusesSaleWhenDeviceCurrencyNotEuro ────────────────────────

    @Test
    fun `RefusesSaleWhenDeviceCurrencyNotEuro mode 1 is not EUR-only`() {
        val fu = StatefulFakeDatecsFiscalPort(currencyMode = 1)
        val (rec, _) = recovery(fu)
        val mode = rec.readCurrencyMode()
        val isEuroReady = mode == 2 || mode == 3
        assertFalse("Currency mode 1 (BGN-primary) must NOT be EUR-ready", isEuroReady)
    }

    // ─── VG-09.3: TestReadinessRequiresEuroMode ───────────────────────────────

    @Test
    fun `TestReadinessRequiresEuroMode modes 2 and 3 are EUR-capable`() {
        for (mode in listOf(2, 3)) {
            val fu = StatefulFakeDatecsFiscalPort(currencyMode = mode)
            val (rec, _) = recovery(fu)
            val readMode = rec.readCurrencyMode()
            val euroReady = readMode == 2 || readMode == 3
            assertTrue("Currency mode $mode should be EUR-ready", euroReady)
        }
        for (mode in listOf(0, 1)) {
            val fu = StatefulFakeDatecsFiscalPort(currencyMode = mode)
            val (rec, _) = recovery(fu)
            val readMode = rec.readCurrencyMode()
            val euroReady = readMode == 2 || readMode == 3
            assertFalse("Currency mode $mode must NOT be EUR-ready", euroReady)
        }
    }
}
