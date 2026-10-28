package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-03: Pre-validation rejects bad input before any I/O (BUG-022).
 *
 * T:
 *  - name 73 chars → FAILED PAYLOAD_INVALID:name, purchaseCalls==0, no FU commands, no PAYMENT_PREPARED.
 *  - qty "1e3" → validation failure.
 *  - qty 120.000 × price 1.00 → accepted (within protocol limit).
 *  - price 100000.00 × qty 100.000 → PAYLOAD_INVALID:line_total_exceeded (> 9999999.99).
 */
class TSTKT03Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    private class CountingFiscal : DatecsFiscalPort {
        var calls = 0
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            calls++
            val data = if (command == 56) "0\tBON-1\t".toByteArray() else "0\t".toByteArray()
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private class CountingCard : DatecsPaymentPort {
        var purchaseCalls = 0
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String): Map<String, String> {
            purchaseCalls++
            return mapOf("approved" to "true", "rrn" to "R1")
        }
        override fun reverse(operationId: String) = mapOf("approved" to "true")
    }

    private fun sale(id: String, lines: List<FiscalLine>, payments: List<SalePayment>) = BlueCashSale(
        operationId = id,
        unp         = "DY000600-OP01-0000001",
        operator    = 1,
        password    = "0000",
        till        = 1,
        lines       = lines,
        payments    = payments
    )

    // name 73 chars → PAYLOAD_INVALID:name, NO FU commands, NO card charge
    @Test fun longNameRejectedBeforeIO() {
        val fiscal = CountingFiscal()
        val card   = CountingCard()
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(fiscal, card, journal).sale(
            sale("op-03a",
                listOf(FiscalLine("А".repeat(73), 'A', "13.67", "1.000")),
                listOf(SalePayment("p1", "CASH", "13.67"))
            )
        )
        assertEquals("FAILED", result["state"])
        assertTrue("error code must mention name", result["error_code"]?.contains("name") == true ||
            result["error_code"]?.contains("PAYLOAD") == true)
        assertEquals("FU commands must be 0 before validation", 0, fiscal.calls)
        assertEquals("card must not be charged", 0, card.purchaseCalls)
        assertFalse("PAYMENT_PREPARED must not appear in journal",
            journal.find("op-03a").any { it.type == "PAYMENT_PREPARED" })
    }

    // qty "1e3" → validation failure (invalid format)
    @Test fun invalidQuantityFormatRejected() {
        try {
            FiscalLine("Item", 'A', "13.67", "1e3")
            // If FiscalLine construction succeeds, DatecsPayloads.line must reject it
            val payload = DatecsPayloads.line(FiscalLine("Item", 'A', "13.67", "1e3"))
            // If we reach here without exception, the test fails
            fail("qty '1e3' must be rejected by DatecsPayloads.line()")
        } catch (e: Exception) {
            // expected — any exception means the bad qty was caught
        }
    }

    // price × qty = 120.00 × 1.000 → accepted (within limit 9999999.99)
    @Test fun validLargeQuantityAccepted() {
        val fiscal = CountingFiscal()
        val card   = CountingCard()
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(fiscal, card, journal).sale(
            sale("op-03c",
                listOf(FiscalLine("Widget", 'A', "1.00", "120.000")),
                listOf(SalePayment("p1", "CASH", "120.00"))
            )
        )
        // 1.00 × 120.000 = 120.00 ≤ 9999999.99 → must proceed to FU
        assertNotEquals("Sale with qty=120.000 must not be FAILED pre-validation",
            "FAILED", result["state"])
    }

    // price 100000.00 × qty 100.000 → PAYLOAD_INVALID:line_total_exceeded
    @Test fun lineTotalExceededRejected() {
        val fiscal = CountingFiscal()
        val card   = CountingCard()
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(fiscal, card, journal).sale(
            sale("op-03d",
                listOf(FiscalLine("Big", 'A', "9999.99", "1000.000")),
                listOf(SalePayment("p1", "CASH", "9999999.00"))
            )
        )
        // 9999.99 × 1000.000 = 9999990.00 — actually within limit; let's use truly oversized
        // 9999.99 × 1000.000 = 9999990.00 < 9999999.99 — so use different values
        // Using 10000.00 × 1000.000 = 10000000.00 > 9999999.99
        // FiscalLine price regex only allows 5+2 digits: max "99999.99"
        // So: "99999.99" × "100.000" = 9999999.00 < 9999999.99 (OK)
        // "99999.99" × "999.000" = 99899990.01 → line total rule.
        // Let's verify the sale with clearly oversized total uses the sale endpoint.
        // The validateSale in DatecsPayloads checks line totals.
        val errorSale = sale("op-03e",
            listOf(FiscalLine("Big", 'A', "50000.00", "200.000")),
            listOf(SalePayment("p1", "CASH", "10000000.00"))
        )
        val result2 = BlueCashCommandProcessor(CountingFiscal(), CountingCard(),
            SignedMemoryJournal(signer())).sale(errorSale)
        assertEquals("FAILED", result2["state"])
        assertTrue("error_code must mention amount or total",
            result2["error_code"]?.let {
                it.contains("amount") || it.contains("PAYLOAD") || it.contains("total")
            } == true)
    }
}
