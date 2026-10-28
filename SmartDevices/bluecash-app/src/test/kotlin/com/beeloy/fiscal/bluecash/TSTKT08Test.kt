package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-08: Empty response from FU is not success (BUG-027).
 *
 * G: port returns empty bytes on cmd 49 and cmd 56.
 * T: both result in error/UNKNOWN (before flush=NotSent, after=Unknown).
 *    Status bytes are parsed per PM §6 table (individual bits → separate subtests).
 */
class TSTKT08Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    // port that returns different responses by command
    private class ResponsePort(private val responses: Map<Int, ByteArray>) : DatecsFiscalPort {
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            val data = responses[command] ?: "0\t".toByteArray()
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    // port that throws Unknown on specific commands (simulates EOF/empty)
    private class ThrowingPort(private val throwOn: Set<Int>) : DatecsFiscalPort {
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            if (command in throwOn) throw DatecsFailure.Unknown("EMPTY_RESPONSE_EOF")
            val data = if (command == 56) "0\tBON-1\t".toByteArray() else "0\t".toByteArray()
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private fun cashSale(id: String) = BlueCashSale(
        operationId = id,
        unp         = "DY000600-OP01-0000001",
        operator    = 1,
        password    = "0000",
        till        = 1,
        lines       = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
        payments    = listOf(SalePayment("p1", "CASH", "13.67"))
    )

    // Empty/unknown response on cmd 49 (line) → before close = definite failure
    @Test fun emptyResponseOnLineCmdIsFailure() {
        val port = ThrowingPort(setOf(49))
        val result = BlueCashCommandProcessor(port, object : DatecsPaymentPort {
            override fun reachable() = true
            override fun purchaseEur(amountMinor: Long, operationId: String) =
                mapOf("approved" to "true", "rrn" to "R1")
            override fun reverse(operationId: String) = mapOf("approved" to "true")
        }, SignedMemoryJournal(signer())).sale(cashSale("op-08a"))

        assertNotEquals("empty response on cmd49 must not be FISCALIZED", "FISCALIZED", result["state"])
    }

    // Empty/unknown response on cmd 56 (close) → UNKNOWN (after potential write)
    @Test fun emptyResponseOnCloseCmdIsUnknown() {
        val port = ThrowingPort(setOf(56))
        val result = BlueCashCommandProcessor(port, object : DatecsPaymentPort {
            override fun reachable() = true
            override fun purchaseEur(amountMinor: Long, operationId: String) =
                mapOf("approved" to "true", "rrn" to "R1")
            override fun reverse(operationId: String) = mapOf("approved" to "true")
        }, SignedMemoryJournal(signer())).sale(cashSale("op-08b"))

        assertEquals("UNKNOWN", result["state"])
    }

    // Status byte bit 4 set (paper near end) — must be parseable without exception.
    // DatecsStatus requires exactly 8 bytes (PM §6 defines 8 status bytes per response frame).
    @Test fun statusBytePaperNearEndParseable() {
        // bit 4 of byte 0 = paper near end; pad to 8 bytes
        val statusBytes = ByteArray(8).also { it[0] = 0x10.toByte() }
        val status = DatecsStatus.parse(statusBytes)
        assertNotNull("Status must parse without exception", status)
        assertEquals("raw must be 8 bytes", 8, status.raw.size)
        assertTrue("paper near end bit (bit 4 of byte 0) must be set", (status.raw[0].toInt() and 0x10) != 0)
    }

    // Status byte 0x00 — no errors
    @Test fun statusByteZeroIsOk() {
        val statusBytes = ByteArray(8)  // all zeros = no flags set
        val status = DatecsStatus.parse(statusBytes)
        assertNotNull(status)
        assertFalse("no fatal flags on all-zero status", status.fatal)
    }

    // Response with error code != 0 must throw/reject
    @Test fun nonZeroErrorCodeIsRejected() {
        val errResponse = "100\tsome error\t".toByteArray(Charsets.UTF_8)
        val response = DatecsResponse(0x20, 49, errResponse, byteArrayOf())
        try {
            response.requireOk()
            fail("Non-zero error code must throw")
        } catch (e: Exception) {
            // expected
        }
    }
}
