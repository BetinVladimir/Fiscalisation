package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger

private class ScriptedFiscalPort(
    private val responses: ArrayDeque<() -> DatecsResponse>
) : DatecsFiscalPort {
    override fun reachable(): Boolean = true
    override fun execute(command: Int, payload: ByteArray): DatecsResponse =
        responses.removeFirst().invoke()
}

private fun noCardPort() = object : DatecsPaymentPort {
    override fun reachable() = true
    override fun purchaseEur(amountMinor: Long, operationId: String) =
        throw UnsupportedOperationException("payment not used")
    override fun reverse(operationId: String) =
        throw UnsupportedOperationException("payment not used")
}

private fun okZResponse(nRep: Int = 42, totA: Long = 1367): DatecsResponse {
    val text = "0\t$nRep\t$totA\t0\t0\t0\t0\t0\t0"
    return DatecsResponse(0x20, 69, text.toByteArray(Charsets.ISO_8859_1), byteArrayOf())
}

private fun timeoutResponse(): Nothing = throw DatecsFailure.Unknown("DATECS_TIMEOUT")

private fun signer() = object : TransactionSigner {
    override val keyId = "test"
    override fun sign(hash: ByteArray) = hash
}

private fun journal() = SignedMemoryJournal(signer())

class DailyReportTest {

    /** Z report that timed out (UNKNOWN), then replayed — must return UNKNOWN (idempotent saga). */
    @Test fun ZRepeatAfterUnknownIsIdempotent() {
        val port1 = ScriptedFiscalPort(ArrayDeque(listOf({ timeoutResponse() })))
        val j = journal()
        val proc1 = BlueCashCommandProcessor(port1, noCardPort(), j)
        val first = proc1.dailyReport("op-z-1", zReport = true)
        assertEquals("UNKNOWN", first["state"])

        // Second attempt — journal already has UNKNOWN; must replay without hitting FU again
        val port2 = ScriptedFiscalPort(ArrayDeque()) // empty: any FU call would throw NoSuchElement
        val proc2 = BlueCashCommandProcessor(port2, noCardPort(), j)
        val second = proc2.dailyReport("op-z-1", zReport = true)
        assertEquals("replay must return UNKNOWN (idempotent)", "UNKNOWN", second["state"])
    }

    /** ERR_Z_REPORT_CLOSED (-104006) must map to FISCALIZED, not FAILED. */
    @Test fun ZAlreadyClosedIsIdempotent() {
        val port = ScriptedFiscalPort(ArrayDeque(listOf({
            throw DatecsFailure.Rejected(-104006, "ERR_Z_REPORT_CLOSED")
        })))
        val proc = BlueCashCommandProcessor(port, noCardPort(), journal())
        val result = proc.dailyReport("op-z-2", zReport = true)
        assertEquals("ERR_Z_REPORT_CLOSED must be FISCALIZED", "FISCALIZED", result["state"])
        assertEquals("Z_ALREADY_CLOSED", result["error_code"])
    }

    /** Z totals stored in journal and result map (nRep, tot_a). */
    @Test fun ZTotalsStoredAndReconciled() {
        val port = ScriptedFiscalPort(ArrayDeque(listOf({ okZResponse(nRep = 7, totA = 1367) })))
        val j = journal()
        val proc = BlueCashCommandProcessor(port, noCardPort(), j)
        val result = proc.dailyReport("op-z-3", zReport = true)

        assertEquals("FISCALIZED", result["state"])
        assertEquals("7", result["n_rep"])
        assertEquals("1367", result["tot_a"])

        val fiscalized = j.find("op-z-3").firstOrNull { it.type == "FISCALIZED" }
        assertNotNull("FISCALIZED record must be in journal", fiscalized)
        val payload = fiscalized!!.payload
        assertTrue("journal payload must contain n_rep", payload.contains("n_rep"))
        assertTrue("journal payload must contain tot_a", payload.contains("tot_a"))
    }

    /** X report preserves report_type=X in result. */
    @Test fun XDoesNotChangeCounters() {
        val port = ScriptedFiscalPort(ArrayDeque(listOf({ okZResponse() })))
        val result = BlueCashCommandProcessor(port, noCardPort(), journal())
            .dailyReport("op-x-1", zReport = false)
        assertEquals("FISCALIZED", result["state"])
        assertEquals("X", result["report_type"])
    }
}
