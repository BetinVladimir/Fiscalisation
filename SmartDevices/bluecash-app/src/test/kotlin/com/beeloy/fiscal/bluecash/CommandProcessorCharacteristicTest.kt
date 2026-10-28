package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * SMS-02.1: Characteristic tests for BlueCashCommandProcessor before shared-runtime extraction.
 *
 * These tests document the CURRENT observable behavior so it can be verified unchanged after
 * the processor strategy is moved into shared-runtime (SMS-02.4).
 *
 * Invariant 1: Money in integer cents (1200¢ → "12.00").
 * Invariant 4: Same operationId → no second document (same Result, no extra port call).
 * Golden receipt: 1200¢ = "12.00", CASH payment.
 */
class CommandProcessorCharacteristicTest {

    // ── Test doubles ──────────────────────────────────────────────────────────

    private fun signer() = object : TransactionSigner {
        override val keyId = "test"
        override fun sign(hash: ByteArray) = hash
    }

    /** Fiscal port that records every command issued and returns minimal OK responses. */
    private class RecordingFiscal : DatecsFiscalPort {
        val commands = mutableListOf<Int>()
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            commands += command
            // Cmd 56 must return a non-blank fiscal reference in field[1] (invariant 6).
            val data = when (command) {
                56 -> DatecsText.encode("0\tBON-1200-01\t")
                else -> DatecsText.encode("0\t")
            }
            return DatecsResponse(0x20, command, data, ByteArray(8))
        }
    }

    /** No-op payment port — cash-only sales do not touch the card terminal. */
    private fun noCashPaymentPort() = object : DatecsPaymentPort {
        override fun reachable() = false
        override fun purchaseEur(amountMinor: Long, operationId: String) =
            error("card should not be called for CASH sale")
        override fun reverse(operationId: String) = error("card reverse not expected")
    }

    // ── Golden sale: 1200¢ = 12.00, one CASH payment ─────────────────────────
    //
    // Invariant 1: unit price "12.00" × qty "1.000" = 1200¢ in integer cent arithmetic:
    //   1200 = (unitPrice_cents × qty_milli + 500) / 1000 = (1200 × 1000 + 500) / 1000 = 1200 ✓

    private fun goldenSale(id: String = "op-1200-01") = BlueCashSale(
        operationId = id,
        unp         = "DY000600-OP01-0000001",
        operator    = 1,
        password    = "0000",
        till        = 1,
        lines       = listOf(FiscalLine("Стока", 'A', "12.00", "1.000")),
        payments    = listOf(SalePayment("p1", "CASH", "12.00"))
    )

    // ── Test 1: SALE 1200¢ cash → state=FISCALIZED with fiscal reference ─────

    @Test fun sale1200CashReturnsFiscalizedWithRef() {
        val fiscal  = RecordingFiscal()
        val journal = SignedMemoryJournal(signer())
        val proc    = BlueCashCommandProcessor(fiscal, noCashPaymentPort(), journal)

        val result = proc.sale(goldenSale())

        // Characteristic: result state is FISCALIZED
        assertEquals(
            "SALE 1200¢ cash must return state=FISCALIZED",
            "FISCALIZED",
            result["state"]
        )
        // Characteristic: fiscal_reference comes from FU response (invariant 6)
        val ref = result["fiscal_reference"]
        assertNotNull("fiscal_reference must be present", ref)
        assertTrue("fiscal_reference must not be blank", ref!!.isNotBlank())
        assertEquals("fiscal_reference must be exactly what FU returned", "BON-1200-01", ref)
    }

    // ── Test 2: Fiscal I/O sequence for CASH sale ─────────────────────────────
    //   Expected: open(48) → line(49) × 1 → payment(53) × 1 → close(56)
    //   No card commands (CASH only).

    @Test fun sale1200CashIssuesExpectedFiscalSequence() {
        val fiscal  = RecordingFiscal()
        val journal = SignedMemoryJournal(signer())
        val proc    = BlueCashCommandProcessor(fiscal, noCashPaymentPort(), journal)

        proc.sale(goldenSale())

        assertEquals(
            "Fiscal I/O sequence: open(48), line(49), payment(53), close(56)",
            listOf(48, 49, 53, 56),
            fiscal.commands
        )
    }

    // ── Test 3: Duplicate operationId → same result, NO second I/O ───────────
    //   Invariant 4: same key → no second document.

    @Test fun duplicateOperationIdReturnsSameResultWithoutSecondIO() {
        val fiscal  = RecordingFiscal()
        val journal = SignedMemoryJournal(signer())
        val proc    = BlueCashCommandProcessor(fiscal, noCashPaymentPort(), journal)

        val result1 = proc.sale(goldenSale("op-dup-01"))
        val commandsAfterFirst = fiscal.commands.size

        val result2 = proc.sale(goldenSale("op-dup-01"))

        // Invariant 4: second call must not emit any new fiscal commands
        assertEquals(
            "Duplicate operationId must not issue new fiscal commands (invariant 4)",
            commandsAfterFirst,
            fiscal.commands.size
        )
        // Same result returned
        assertEquals("state must match on replay", result1["state"], result2["state"])
        assertEquals(
            "fiscal_reference must match on replay",
            result1["fiscal_reference"],
            result2["fiscal_reference"]
        )
    }

    // ── Test 4: Journal records ACCEPTED → EXECUTING → FISCALIZED ─────────────
    //   Characteristic: journal entry types document the durable saga states.

    @Test fun sale1200CashRecordsJournalStateSequence() {
        val fiscal  = RecordingFiscal()
        val journal = SignedMemoryJournal(signer())
        val proc    = BlueCashCommandProcessor(fiscal, noCashPaymentPort(), journal)

        proc.sale(goldenSale("op-journal-01"))

        val entries = journal.find("op-journal-01").map { it.type }
        assertTrue("ACCEPTED must be in journal", "ACCEPTED" in entries)
        assertTrue("EXECUTING must be in journal", "EXECUTING" in entries)
        assertTrue("FISCALIZED must be final journal state", "FISCALIZED" in entries)
        // ACCEPTED comes before EXECUTING which comes before FISCALIZED
        val acceptedIdx   = entries.indexOf("ACCEPTED")
        val executingIdx  = entries.indexOf("EXECUTING")
        val fiscalizedIdx = entries.indexOf("FISCALIZED")
        assertTrue("ACCEPTED < EXECUTING", acceptedIdx < executingIdx)
        assertTrue("EXECUTING < FISCALIZED", executingIdx < fiscalizedIdx)
    }

    // ── Test 5: Idempotency key is checked BEFORE fiscal I/O ─────────────────
    //   Characteristic: on replay the lock is released immediately after journal lookup.

    @Test fun replayBeforeAnyFiscalIO() {
        val fiscal  = RecordingFiscal()
        val journal = SignedMemoryJournal(signer())
        val proc    = BlueCashCommandProcessor(fiscal, noCashPaymentPort(), journal)

        proc.sale(goldenSale("op-idem-01"))
        val firstCallCommands = fiscal.commands.toList()

        // Second call: same operationId, should hit journal cache before fiscal I/O
        proc.sale(goldenSale("op-idem-01"))

        assertEquals(
            "Second call must not add fiscal commands (idempotency guard fires before I/O)",
            firstCallCommands,
            fiscal.commands
        )
    }
}
