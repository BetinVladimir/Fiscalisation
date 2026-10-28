package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-02: Card is NOT reversed on UNKNOWN fiscal result (BUG-021).
 *
 * G: CARD 8.67 APPROVE; eofAfter(56).
 * W: sale().
 * T: reverseCalls==0; result FISCAL_RESULT_UNKNOWN/RECOVERY_REQUIRED(card_state=APPROVED).
 *
 * Variant B: rejectAt(56, -111) (definite failure) → reverseCalls==1, journal CARD_COMPENSATED.
 */
class TSTKT02Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    // fiscal port with scripted responses
    private class ScriptPort(
        private val script: MutableList<() -> DatecsResponse>
    ) : DatecsFiscalPort {
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse =
            script.removeAt(0).invoke()
    }

    private fun okResponse(cmd: Int) = DatecsResponse(0x20, cmd,
        (if (cmd == 56) "0\tBON-42\t" else "0\t").toByteArray(), byteArrayOf())

    // Card port that counts reversal calls
    private class CountingCard : DatecsPaymentPort {
        var approveCalls = 0
        var reverseCalls = 0
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String): Map<String, String> {
            approveCalls++
            return mapOf("approved" to "true", "rrn" to "R1")
        }
        override fun reverse(operationId: String): Map<String, String> {
            reverseCalls++
            return mapOf("approved" to "true")
        }
    }

    // Golden split: CASH 500¢=5.00, CARD 867¢=8.67, total 1367¢=13.67
    private fun cardSale(id: String = "op-02") = BlueCashSale(
        operationId = id,
        unp         = "DY000600-OP01-0000001",
        operator    = 1,
        password    = "0000",
        till        = 1,
        lines       = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
        payments    = listOf(SalePayment("p-card", "CARD", "13.67"))
    )

    // T: reverseCalls==0 when fiscal result is UNKNOWN (EOF after cmd56)
    @Test fun cardNotReversedOnFiscalUnknown() {
        val script = mutableListOf<() -> DatecsResponse>(
            { okResponse(48) },  // cmd 48 open
            { okResponse(49) },  // cmd 49 line
            { okResponse(53) },  // cmd 53 payment
            { throw DatecsFailure.Unknown("DATECS_EOF") }  // cmd 56 unknown
        )
        val card = CountingCard()
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(ScriptPort(script), card, journal).sale(cardSale())

        // Engine returns RECOVERY_REQUIRED (not UNKNOWN) when card was approved but fiscal is unclear.
        // Invariant 2: no card reversal when fiscal outcome is unknown.
        assertEquals("RECOVERY_REQUIRED", result["state"])
        assertEquals("card must NOT be reversed on UNKNOWN fiscal", 0, card.reverseCalls)
        assertEquals("card must have been approved once", 1, card.approveCalls)
    }

    // Variant B: definite failure after card approval → card reversed, CARD_COMPENSATED
    @Test fun cardReversedOnDefiniteFailure() {
        val script = mutableListOf<() -> DatecsResponse>(
            { okResponse(48) },
            { okResponse(49) },
            { okResponse(53) },
            { throw DatecsFailure.Rejected(-111, "-111") },  // definite failure on cmd 56
            { okResponse(60) }  // cmd 60 cancel receipt — must succeed so compensation proceeds
        )
        val card = CountingCard()
        val journal = SignedMemoryJournal(signer())
        val result = BlueCashCommandProcessor(ScriptPort(script), card, journal).sale(cardSale("op-02b"))

        assertEquals("COMPENSATED", result["state"])
        assertEquals("card must be reversed on definite failure", 1, card.reverseCalls)
        assertTrue("CARD_COMPENSATED must be journaled",
            journal.find("op-02b").any { it.type.contains("COMPENSATED") || it.type.contains("CARD") })
    }
}
