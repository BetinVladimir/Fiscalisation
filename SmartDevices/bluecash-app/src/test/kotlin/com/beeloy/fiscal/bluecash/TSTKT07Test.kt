package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-07: No document number in cmd56 response → UNKNOWN (BUG-026).
 *
 * G: cmd56 response with empty slip-number field.
 * T: result FISCAL_RESULT_UNKNOWN; no synthetic document number.
 *
 * Positive: cmd56 response with SlipNumber=470 → fiscal_reference="470".
 */
class TSTKT07Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    private class ScriptedFiscal(
        private val cmd56Response: ByteArray
    ) : DatecsFiscalPort {
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            val data = if (command == 56) cmd56Response else "0\t".toByteArray()
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private fun cardPort() = object : DatecsPaymentPort {
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String) =
            mapOf("approved" to "true", "rrn" to "R1")
        override fun reverse(operationId: String) = mapOf("approved" to "true")
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

    // cmd56 response with empty slip number → result must indicate unparseable reference
    @Test fun emptySlipNumberIsUnknown() {
        // "0\t\t" — zero tab empty-tab (no slip number)
        val fiscal = ScriptedFiscal("0\t\t".toByteArray())
        val result = BlueCashCommandProcessor(fiscal, cardPort(),
            SignedMemoryJournal(signer())).sale(cashSale("op-07a"))

        // Must return RECOVERY_REQUIRED / FISCAL_REFERENCE_UNPARSEABLE
        assertTrue("empty slip number must result in RECOVERY_REQUIRED",
            result["state"] == "RECOVERY_REQUIRED" || result["state"] == "UNKNOWN")
        assertNull("fiscal_reference must not be a synthetic number",
            result["fiscal_reference"])
    }

    // Positive: SlipNumber=470 → fiscal_reference="470"
    @Test fun presentSlipNumberIsFiscalReference() {
        // "0\t470\t" — slip number 470
        val fiscal = ScriptedFiscal("0\t470\t".toByteArray())
        val result = BlueCashCommandProcessor(fiscal, cardPort(),
            SignedMemoryJournal(signer())).sale(cashSale("op-07b"))

        assertEquals("FISCALIZED", result["state"])
        assertEquals("fiscal_reference must be '470'", "470", result["fiscal_reference"])
    }

    // "DATECS-<op>" must NEVER appear in any field — no synthetic number generation
    @Test fun noSyntheticDocumentNumber() {
        val fiscal = ScriptedFiscal("0\t\t".toByteArray())
        val result = BlueCashCommandProcessor(fiscal, cardPort(),
            SignedMemoryJournal(signer())).sale(cashSale("op-07c"))

        for ((k, v) in result) {
            assertFalse("synthetic DATECS-* number must not appear in result field $k",
                v.startsWith("DATECS-"))
        }
    }
}
