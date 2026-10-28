package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

/**
 * REFUND-01 recovery invariants:
 *   Invariant 2 – UNKNOWN ≠ FAILED: in-progress journal entry → UNKNOWN/RECOVERY_REQUIRED, no card refund.
 *   Invariant 3 – Storno-first: fiscal storno confirmed before card reversal.
 *   Invariant 4 – Idempotency: same operationId returns cached terminal state.
 */
class RefundRecoveryTest {

    // ── fixtures ─────────────────────────────────────────────────────────────────

    private val stornoDoc = DatecsStornoDocument(
        reason = 0,
        documentNumber = 428,
        documentDateTime = "24-04-19 08:36:27",
        fiscalMemoryNumber = "02636571",
        originalUnp = "DT636497-0021-0010001"
    )

    private fun reversal(
        operationId: String = "rev-001",
        payments: List<SalePayment> = listOf(SalePayment("p-card-1", "CARD", "8.67"))
    ) = BlueCashReversal(
        operationId = operationId,
        originalOperationId = "op-001",
        operator = 1,
        password = "0000",
        till = 1,
        originalDocument = stornoDoc,
        lines = listOf(FiscalLine("Item", 'A', "8.67", "1.000")),
        payments = payments
    )

    private fun makeJournal() = SignedMemoryJournal(
        object : TransactionSigner {
            override val keyId = "test-key"
            override fun sign(hash: ByteArray) = hash
        },
        { java.time.Instant.now() }
    )

    private fun makeProcessor(
        fiscal: DatecsFiscalPort,
        card: DatecsPaymentPort = MockPaymentPort(),
        journal: SignedMemoryJournal = makeJournal()
    ) = BlueCashCommandProcessor(fiscal, card, journal)

    // ── test 1: storno times out (UNKNOWN) → no card reversal attempted ──────────

    @Test fun `storno_unknown_no_card_refund_attempted`() {
        val trackingCard = TrackingPaymentPort()
        val fiscal = MockFiscalPort(DatecsFailure.Unknown("DATECS_EOF"))
        val processor = makeProcessor(fiscal, trackingCard)

        val result = processor.reverse(reversal("rev-unknown-001"))

        assertEquals("UNKNOWN", result["state"])
        assertEquals(0, trackingCard.reverseCallCount)
    }

    // ── test 2: in-flight (ACCEPTED+EXECUTING in journal) → UNKNOWN/RECOVERY ────

    @Test fun `in_flight_journal_returns_recovery_required`() {
        val journal = makeJournal()
        journal.append("rev-inflight-001", "ACCEPTED", "state=ACCEPTED")
        journal.append("rev-inflight-001", "EXECUTING", "state=EXECUTING")

        val processor = makeProcessor(MockFiscalPort(), journal = journal)
        val result = processor.reverse(reversal("rev-inflight-001"))

        assertEquals("UNKNOWN", result["state"])
        assertEquals("RECOVERY_REQUIRED", result["error_code"])
    }

    // ── test 3: REFUND_PENDING in journal → idempotent (cached) return ───────────

    @Test fun `refund_pending_is_idempotent`() {
        val decliningCard = DecliningPaymentPort()
        val journal = makeJournal()
        val processor = makeProcessor(MockFiscalPort(), decliningCard, journal)

        // First call: fiscal succeeds, card declines → REFUND_PENDING written
        val first = processor.reverse(reversal("rev-pending-001"))
        assertEquals("REFUND_PENDING", first["state"])

        // Second call: different card port (would now approve) → must return cached REFUND_PENDING
        val trackingCard = TrackingPaymentPort()
        val processor2 = makeProcessor(MockFiscalPort(), trackingCard, journal)
        val second = processor2.reverse(reversal("rev-pending-001"))

        assertEquals("REFUND_PENDING", second["state"])
        assertEquals(0, trackingCard.reverseCallCount)
    }

    // ── test 4: REVERSED in journal → idempotent (cached) return ─────────────────

    @Test fun `reversed_is_idempotent`() {
        val journal = makeJournal()
        val processor = makeProcessor(MockFiscalPort(), MockPaymentPort(), journal)

        val first = processor.reverse(reversal("rev-done-001"))
        assertEquals("REVERSED", first["state"])

        // Second call with failing fiscal: must return cached REVERSED, not re-execute
        val failingFiscal = MockFiscalPort(DatecsFailure.Unknown("gone"))
        val processor2 = makeProcessor(failingFiscal, MockPaymentPort(), journal)
        val second = processor2.reverse(reversal("rev-done-001"))

        assertEquals("REVERSED", second["state"])
    }

    // ── test 5: multi-CARD split → two REFUND_PREPARED journal entries ────────────

    @Test fun `multi_card_payments_each_get_refund_prepared_entry`() {
        val journal = makeJournal()
        val processor = makeProcessor(MockFiscalPort(), MockPaymentPort(), journal)

        val twoCardReversal = reversal(
            "rev-multi-001",
            payments = listOf(
                SalePayment("p-card-1", "CARD", "5.00"),
                SalePayment("p-card-2", "CARD", "3.67")
            )
        )
        val result = processor.reverse(twoCardReversal)
        assertEquals("REVERSED", result["state"])

        val preparedEntries = journal.find("rev-multi-001").filter { it.type == "REFUND_PREPARED" }
        assertEquals(2, preparedEntries.size)
        assertNotEquals(
            "Each CARD payment must get its own REFUND_PREPARED entry",
            preparedEntries[0].payload,
            preparedEntries[1].payload
        )
    }
}

// ── test doubles (new; MockFiscalPort and MockPaymentPort live in BlueCashEngineUnknownTest.kt) ──

/** Records how many times reverse() was called. */
class TrackingPaymentPort : DatecsPaymentPort {
    var reverseCallCount = 0
    override fun purchaseEur(amountMinor: Long, operationId: String) =
        mapOf("approved" to "true", "rrn" to "RRN123", "authorization_code" to "AC456")
    override fun reverse(operationId: String): Map<String, String> {
        reverseCallCount++
        return mapOf("approved" to "true", "rrn" to "RRN999", "authorization_code" to "AC000")
    }
    override fun reachable() = true
}

/** Always declines card reversals (simulates card-decline / terminal-offline). */
class DecliningPaymentPort : DatecsPaymentPort {
    override fun purchaseEur(amountMinor: Long, operationId: String) =
        mapOf("approved" to "false")
    override fun reverse(operationId: String) =
        mapOf("approved" to "false", "error_code" to "CARD_DECLINED")
    override fun reachable() = true
}
