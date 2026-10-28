package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-05: Retry after crash returns UNKNOWN — no new FU commands (BUG-024).
 *
 * G: journal contains ACCEPTED+EXECUTING for an operation.
 * W: retry execute same operationId.
 * T: result FISCAL_RESULT_UNKNOWN (not SALE_ALREADY_EXISTS); unp.next not called;
 *    no FU commands on retry (status lookup only if required).
 */
class TSTKT05Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    private class TrackingFiscal : DatecsFiscalPort {
        val calls = mutableListOf<Int>()
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            calls += command
            val data = if (command == 56) "0\tBON-42\t".toByteArray() else "0\t".toByteArray()
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private fun cardPort() = object : DatecsPaymentPort {
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String) =
            mapOf("approved" to "true", "rrn" to "R1")
        override fun reverse(operationId: String) = mapOf("approved" to "true")
    }

    private fun sale(id: String) = BlueCashSale(
        operationId = id,
        unp         = "DY000600-OP01-0000001",
        operator    = 1,
        password    = "0000",
        till        = 1,
        lines       = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
        payments    = listOf(SalePayment("p1", "CASH", "13.67"))
    )

    // G: journal has ACCEPTED+EXECUTING (simulates crash mid-execution)
    // W: retry with same operationId
    // T: result UNKNOWN (RECOVERY_REQUIRED), no new FU commands
    @Test fun retryAfterCrashReturnsUnknown() {
        val journal = SignedMemoryJournal(signer())
        // Pre-populate journal to simulate crash state: ACCEPTED then EXECUTING
        journal.append("op-05", "ACCEPTED", "state=ACCEPTED")
        journal.append("op-05", "EXECUTING", "state=EXECUTING")

        val fiscal = TrackingFiscal()
        val proc = BlueCashCommandProcessor(fiscal, cardPort(), journal)

        val callsBefore = fiscal.calls.size
        val result = proc.sale(sale("op-05"))

        // Must return UNKNOWN / RECOVERY_REQUIRED — not attempt to run again
        assertTrue("result must be UNKNOWN or RECOVERY_REQUIRED when ACCEPTED+EXECUTING in journal",
            result["state"] == "UNKNOWN" || result["error_code"] == "RECOVERY_REQUIRED")
        // No new FU commands should have been issued (or very minimal)
        val newCalls = fiscal.calls.size - callsBefore
        assertTrue("retry must not issue regular sale FU commands (got $newCalls new calls)",
            newCalls == 0)
    }

    // Idempotency: same operation already FISCALIZED → return cached result
    @Test fun completedSaleReturnsCachedResult() {
        val fiscal = TrackingFiscal()
        val journal = SignedMemoryJournal(signer())
        val proc = BlueCashCommandProcessor(fiscal, cardPort(), journal)

        // First call: success
        val first = proc.sale(sale("op-05b"))
        assertEquals("FISCALIZED", first["state"])

        val callsAfterFirst = fiscal.calls.size

        // Retry with same operationId: must return same result, no new FU commands
        val second = proc.sale(sale("op-05b"))
        assertEquals("FISCALIZED", second["state"])
        assertEquals("fiscal_reference", first["fiscal_reference"], second["fiscal_reference"])
        assertEquals("retry must not issue new FU commands", callsAfterFirst, fiscal.calls.size)
    }
}
