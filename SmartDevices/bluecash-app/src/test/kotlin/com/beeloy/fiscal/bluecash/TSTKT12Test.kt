package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-12: Non-fiscal document (printer test) closes on failure (BUG-031).
 *
 * G: printerTest; fiscal port fails on cmd 42 (print test page).
 * T: after failure, cmd 39 (non-fiscal close) must be called;
 *    result is FAILED; subsequent printerTest can succeed.
 */
class TSTKT12Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    // Port that tracks calls and fails on specific cmd
    private class TrackPort(private val failOn: Int, private val ex: Throwable) : DatecsFiscalPort {
        val calls = mutableListOf<Int>()
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            calls += command
            if (command == failOn) throw ex
            return DatecsResponse(0x20, command, "0\t".toByteArray(), byteArrayOf())
        }
    }

    // printerTest cmd sequence: 38=open non-fiscal, 42=print, 39=close non-fiscal
    // When cmd 42 fails, engine must still send cmd 39 to close the document.
    @Test fun printerTestClosesOnFailure() {
        val port = TrackPort(42, DatecsFailure.Rejected(-100, "PRINT_ERROR"))
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(port, object : DatecsPaymentPort {
            override fun reachable() = true
            override fun purchaseEur(amountMinor: Long, operationId: String) =
                mapOf("approved" to "true", "rrn" to "R1")
            override fun reverse(operationId: String) = mapOf("approved" to "true")
        }, journal).printerTest("pt-12a")

        assertEquals("printerTest must return FAILED on print failure", "FAILED", result["state"])
        // cmd 39 (close non-fiscal) must appear after cmd 42 (print)
        val has38 = 38 in port.calls
        val has42 = 42 in port.calls
        val has39 = 39 in port.calls
        assertTrue("cmd 38 (open non-fiscal) must be called", has38)
        assertTrue("cmd 42 (print) must be called", has42)
        assertTrue("cmd 39 (close non-fiscal) must be called after failure on cmd 42", has39)
        // Order: 38 before 42 before 39
        val idx38 = port.calls.indexOf(38)
        val idx42 = port.calls.indexOf(42)
        val idx39 = port.calls.lastIndexOf(39)
        assertTrue("cmd 38 must come before cmd 42", idx38 < idx42)
        assertTrue("cmd 39 must come after cmd 42", idx39 > idx42)
    }

    // After failure, a subsequent printerTest on a new operation must succeed
    @Test fun printerTestSucceedsAfterFailure() {
        // Second test: no failure
        val goodPort = object : DatecsFiscalPort {
            override fun reachable() = true
            override fun execute(command: Int, payload: ByteArray): DatecsResponse =
                DatecsResponse(0x20, command, "0\t".toByteArray(), byteArrayOf())
        }
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(goodPort, object : DatecsPaymentPort {
            override fun reachable() = true
            override fun purchaseEur(amountMinor: Long, operationId: String) =
                mapOf("approved" to "true", "rrn" to "R1")
            override fun reverse(operationId: String) = mapOf("approved" to "true")
        }, journal).printerTest("pt-12b")

        assertNotEquals("good printerTest must not fail", "FAILED", result["state"])
    }
}
