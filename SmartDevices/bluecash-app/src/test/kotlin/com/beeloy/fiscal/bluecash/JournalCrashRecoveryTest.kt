package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-DEEP-06: Android journal crash recovery.
 *
 * Invariant 2: UNKNOWN ≠ FAILED — after timeout/crash, no repeat write, no auto card refund.
 * Invariant 3: Storno-first — card refund only after confirmed fiscal storno.
 * Invariant 4: Same key → no second document.
 *
 * Tests: after ACCEPTED→EXECUTING is written and process is killed (simulated by constructing
 * a new BlueCashCommandProcessor with a pre-loaded journal), the second invocation must return
 * RECOVERY_REQUIRED, never re-execute the fiscal command.
 */
class JournalCrashRecoveryTest {

    private fun signer() = object : TransactionSigner {
        override val keyId = "test"
        override fun sign(hash: ByteArray) = hash
    }

    private class CountingFiscalPort : DatecsFiscalPort {
        var calls = 0
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            calls++
            // Simulate device response for any command
            val data = when (command) {
                48 -> "0\t".toByteArray()  // open receipt
                56 -> "0\t1\t".toByteArray()  // total
                else -> "0\t".toByteArray()
            }
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private fun noCard() = object : DatecsPaymentPort {
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String) =
            mapOf("approved" to "true", "rrn" to "RRN-01")
        override fun reverse(operationId: String) = mapOf("approved" to "true")
    }

    private fun goldenSale(id: String) = BlueCashSale(
        operationId = id,
        unp = "DY000600-OP01-0000001",
        operator = 1,
        password = "0000",
        till = 1,
        lines = listOf(
            FiscalLine("Кафе", 'A', "12.00", "1.000"),
            FiscalLine("Захар", 'A', "1.67", "1.000")
        ),
        payments = listOf(SalePayment("p1", "CASH", "13.67"))
    )

    // ── Core crash-recovery test ───────────────────────────────────────────────

    /**
     * Simulate: process killed after ACCEPTED+EXECUTING were written but before fiscal response.
     * On restart (same journal, new processor), the command must return RECOVERY_REQUIRED.
     */
    @Test fun `crash after EXECUTING returns RECOVERY_REQUIRED on restart`() {
        val journal = SignedMemoryJournal(signer())
        // Simulate a crash: manually write ACCEPTED and EXECUTING without a terminal state
        journal.append("op-crash-1", "ACCEPTED", "state=ACCEPTED")
        journal.append("op-crash-1", "EXECUTING", "state=EXECUTING")

        // "Restart": new processor instance, same durable journal
        val fiscal2 = CountingFiscalPort()
        val proc2 = BlueCashCommandProcessor(fiscal2, noCard(), journal)
        val result = proc2.administrative("op-crash-1", 69, byteArrayOf())

        assertEquals("Must return RECOVERY_REQUIRED after crash at EXECUTING",
            "RECOVERY_REQUIRED", result["error_code"])
        assertEquals("State must be UNKNOWN", "UNKNOWN", result["state"])
        // Invariant 4: no second document — fiscal device must NOT be called again
        assertEquals("Fiscal device must not be re-called on RECOVERY_REQUIRED", 0, fiscal2.calls)
    }

    @Test fun `crash after ACCEPTED (only) also returns RECOVERY_REQUIRED`() {
        val journal = SignedMemoryJournal(signer())
        journal.append("op-crash-2", "ACCEPTED", "state=ACCEPTED")

        val fiscal2 = CountingFiscalPort()
        val proc2 = BlueCashCommandProcessor(fiscal2, noCard(), journal)
        val result = proc2.administrative("op-crash-2", 69, byteArrayOf())

        assertEquals("RECOVERY_REQUIRED", result["error_code"])
        assertEquals("UNKNOWN", result["state"])
        assertEquals("No re-execution after ACCEPTED-only crash", 0, fiscal2.calls)
    }

    // ── Sale-level crash recovery ─────────────────────────────────────────────

    @Test fun `sale crash after EXECUTING returns RECOVERY_REQUIRED on replay`() {
        val journal = SignedMemoryJournal(signer())
        // Write the journal entries that a crashed sale process would leave
        journal.append("op-sale-crash", "ACCEPTED", "state=ACCEPTED")
        journal.append("op-sale-crash", "EXECUTING", "state=EXECUTING")

        val fiscal2 = CountingFiscalPort()
        val proc2 = BlueCashCommandProcessor(fiscal2, noCard(), journal)
        val result = proc2.sale(goldenSale("op-sale-crash"))

        assertEquals("RECOVERY_REQUIRED", result["error_code"])
        assertEquals("UNKNOWN", result["state"])
        assertEquals("No new fiscal commands after EXECUTING crash", 0, fiscal2.calls)
    }

    // ── Normal idempotency: completed ops replay their result ─────────────────

    @Test fun `completed FISCALIZED sale replays without re-execution`() {
        val journal = SignedMemoryJournal(signer())
        val fiscal1 = CountingFiscalPort()
        val proc1 = BlueCashCommandProcessor(fiscal1, noCard(), journal)
        // First call: succeeds
        val r1 = proc1.sale(goldenSale("op-ok-1"))
        assertEquals("FISCALIZED", r1["state"])
        val callsAfterFirst = fiscal1.calls

        // Second call with same operationId (idempotency replay)
        val fiscal2 = CountingFiscalPort()
        val proc2 = BlueCashCommandProcessor(fiscal2, noCard(), journal)
        val r2 = proc2.sale(goldenSale("op-ok-1"))
        assertEquals("FISCALIZED", r2["state"])
        assertEquals("No re-execution on idempotent replay", 0, fiscal2.calls)
        assertEquals("callsAfterFirst unchanged", callsAfterFirst, fiscal1.calls)
    }

    // ── Journal entry sequence invariant ─────────────────────────────────────

    @Test fun `journal records ACCEPTED before EXECUTING before terminal state`() {
        val journal = SignedMemoryJournal(signer())
        val proc = BlueCashCommandProcessor(CountingFiscalPort(), noCard(), journal)
        proc.sale(goldenSale("op-sequence"))

        val events = journal.find("op-sequence")
        val types = events.map { it.type }
        val acceptedIdx = types.indexOf("ACCEPTED")
        val executingIdx = types.indexOf("EXECUTING")
        assertTrue("ACCEPTED must exist", acceptedIdx >= 0)
        assertTrue("EXECUTING must exist", executingIdx >= 0)
        assertTrue("ACCEPTED before EXECUTING", acceptedIdx < executingIdx)
        // A terminal state must come after EXECUTING
        val terminalIdx = types.indexOfFirst { it in setOf("FISCALIZED", "FAILED", "UNKNOWN", "RECOVERY_REQUIRED") }
        assertTrue("Terminal state must exist", terminalIdx >= 0)
        assertTrue("EXECUTING before terminal", executingIdx < terminalIdx)
    }

    // ── No auto card refund on UNKNOWN (invariant 2) ──────────────────────────

    @Test fun `UNKNOWN outcome does not trigger automatic card refund`() {
        // Simulate a sale that enters UNKNOWN state (e.g. timeout after EXECUTING)
        val journal = SignedMemoryJournal(signer())
        var cardReversals = 0
        val card = object : DatecsPaymentPort {
            override fun reachable() = true
            override fun purchaseEur(amountMinor: Long, operationId: String) =
                mapOf("approved" to "true", "rrn" to "RRN-02")
            override fun reverse(operationId: String): Map<String, String> {
                cardReversals++
                return mapOf("approved" to "true")
            }
        }

        // Pre-populate journal with ACCEPTED+EXECUTING (simulating crash)
        journal.append("op-unknown-1", "ACCEPTED", "state=ACCEPTED")
        journal.append("op-unknown-1", "EXECUTING", "state=EXECUTING")

        val proc = BlueCashCommandProcessor(CountingFiscalPort(), card, journal)
        val result = proc.sale(goldenSale("op-unknown-1"))

        // Result is RECOVERY_REQUIRED (UNKNOWN state) — no card refund
        assertEquals("UNKNOWN", result["state"])
        assertEquals("No auto card refund on UNKNOWN", 0, cardReversals)
    }
}
