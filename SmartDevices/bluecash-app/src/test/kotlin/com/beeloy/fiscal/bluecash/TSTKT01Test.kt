package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-01: Open receipt is cancelled when a sale fails (BUG-020).
 *
 * G: fiscal port. W: sale().
 * T: sequence [48, 49✗, CANCEL]; journal: RECEIPT_CANCELLED; result FAILED(code=-111);
 *    next sale() succeeds (receipt not blocked).
 * Variant 2: rejectAt(53, -112) → [48, 49, 53✗, CANCEL].
 * Variant 3: cancel also fails → RECEIPT_CANCEL_FAILED, result RECOVERY_REQUIRED.
 */
class TSTKT01Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    private class FailAtPort(private val failMap: Map<Int, Throwable>) : DatecsFiscalPort {
        val calls = mutableListOf<Int>()
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            calls += command
            failMap[command]?.let { throw it }
            val data = if (command == 56) "0\tBON-42\t".toByteArray() else "0\t".toByteArray()
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private fun noCard() = object : DatecsPaymentPort {
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String) =
            mapOf("approved" to "true", "rrn" to "R1")
        override fun reverse(operationId: String) = mapOf("approved" to "true")
    }

    // Golden receipt: 1367¢ = 1200 + 167; total 13.67, CASH 13.67
    private fun goldenSale(id: String = "op-01") = BlueCashSale(
        operationId = id,
        unp         = "DY000600-OP01-0000001",
        operator    = 1,
        password    = "0000",
        till        = 1,
        lines       = listOf(
            FiscalLine("Кафе",  'A', "12.00", "1.000"),
            FiscalLine("Захар", 'A', "1.67",  "1.000")
        ),
        payments    = listOf(SalePayment("p1", "CASH", "13.67"))
    )

    // T: sequence [48, 49✗, CANCEL]; journal RECEIPT_CANCELLED; FAILED code=-111
    @Test fun openReceiptCancelledOnLineFailure() {
        val port = FailAtPort(mapOf(49 to DatecsFailure.Rejected(-111, "-111")))
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(port, noCard(), journal).sale(goldenSale())

        assertEquals(listOf(48, 49, DatecsPayloads.CMD_CANCEL_RECEIPT), port.calls)
        assertEquals("FAILED", result["state"])
        assertEquals("-111", result["error_code"])
        assertTrue("RECEIPT_CANCELLED must be journaled",
            journal.find("op-01").any { it.type == "RECEIPT_CANCELLED" })
    }

    // Variant 2: rejectAt(53, -112) → [48, 49, 53✗, CANCEL]
    @Test fun openReceiptCancelledOnPaymentFailure() {
        val port = FailAtPort(mapOf(53 to DatecsFailure.Rejected(-112, "-112")))
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(port, noCard(), journal).sale(goldenSale("op-02"))

        // 2 FiscalLines → cmd 49 sent twice (once per line)
        assertEquals(listOf(48, 49, 49, 53, DatecsPayloads.CMD_CANCEL_RECEIPT), port.calls)
        assertEquals("FAILED", result["state"])
        assertEquals("-112", result["error_code"])
        assertTrue(journal.find("op-02").any { it.type == "RECEIPT_CANCELLED" })
    }

    // Variant 3: cancel also fails → RECEIPT_CANCEL_FAILED, RECOVERY_REQUIRED
    @Test fun cancelFailureIsRecoveryRequired() {
        val port = FailAtPort(mapOf(
            49 to DatecsFailure.Rejected(-111, "-111"),
            DatecsPayloads.CMD_CANCEL_RECEIPT to DatecsFailure.Unknown("DATECS_EOF")
        ))
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(port, noCard(), journal).sale(goldenSale("op-03"))

        assertEquals("RECOVERY_REQUIRED", result["state"])
        assertTrue(journal.find("op-03").any { it.type == "RECEIPT_CANCEL_FAILED" })
    }

    // T: next sale() succeeds (receipt not permanently blocked)
    @Test fun nextSaleSucceedsAfterCancelledReceipt() {
        // First sale fails; cancel succeeds.
        val portA = FailAtPort(mapOf(49 to DatecsFailure.Rejected(-111, "-111")))
        val journal = SignedMemoryJournal(signer())
        val proc = BlueCashCommandProcessor(portA, noCard(), journal)
        val fail1 = proc.sale(goldenSale("op-04a"))
        assertEquals("FAILED", fail1["state"])

        // Second sale (different operationId) must succeed.
        val portB = FailAtPort(emptyMap())
        val proc2 = BlueCashCommandProcessor(portB, noCard(), journal)
        val ok = proc2.sale(goldenSale("op-04b"))
        assertEquals("FISCALIZED", ok["state"])
    }
}
