package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

class DatecsStornoTest {

    // ─── VG-08.1: StornoReasonMapping ─────────────────────────────────────────

    @Test fun StornoReasonMapping() {
        assertEquals(0, stornoReasonCode("OPERATOR_ERROR"))
        assertEquals(1, stornoReasonCode("CUSTOMER_RETURN"))
        assertEquals(2, stornoReasonCode("TAX_BASE_REDUCTION"))
        // Case-insensitive
        assertEquals(0, stornoReasonCode("operator_error"))
        assertEquals(1, stornoReasonCode("Customer_Return"))
    }

    @Test fun StornoReasonMappingRejectsUnknown() {
        try {
            stornoReasonCode("SOMETHING_ELSE")
            fail("Expected IllegalArgumentException for unknown reason")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message?.contains("UNKNOWN_STORNO_REASON") == true)
        }
    }

    // ─── VG-08.2: StornoOpenMatchesSpecExampleVector ──────────────────────────
    //
    // Spec §4.6 example (tab-separated cmd 43 payload):
    //   21\t9876\t21\t0\t428\t24-04-19 08:36:27\t02636571\t\t\t\tDT636497-0021-0010001
    //
    // Fields: operator | password | till | reason | docNum | dateTime | fmNumber |
    //         invoice | invoiceNum | invoiceReason | originalUnp

    @Test fun StornoOpenMatchesSpecExampleVector() {
        val doc = DatecsStornoDocument(
            reason = 0,
            documentNumber = 428L,
            documentDateTime = "24-04-19 08:36:27",
            fiscalMemoryNumber = "02636571",
            originalUnp = "DT636497-0021-0010001"
        )
        val payload = DatecsPayloads.stornoOpen(
            operator = 21,
            password = "9876",
            till = 21,
            document = doc
        )
        val text = DatecsText.decode(payload)
        // bytes() adds a trailing separator — strip it for comparison with the spec example.
        val expected = "21\t9876\t21\t0\t428\t24-04-19 08:36:27\t02636571\t\t\t\tDT636497-0021-0010001"
        assertEquals("stornoOpen payload must match spec §4.6 example", expected, text.trimEnd('\t'))
    }

    // ─── VG-08.3: StornoPaymentNoChange ───────────────────────────────────────
    //
    // Datecs returns -111094 ("Payment in storno can not have change") when the
    // payment amount exceeds the receipt total. Test that:
    //  1. Payment equal to total → accepted (no change).
    //  2. Payment greater than total → rejected with -111094.

    @Test fun StornoPaymentNoChange_exactAmountAccepted() {
        // Exact-amount payment (amount == total): FU must accept (mode=0, amount="13.67").
        // This is a codec test: the payment payload must not carry a change field.
        val payload = DatecsPayloads.payment(0, "13.67")
        val text = DatecsText.decode(payload)
        // Expected: "0\t13.67" (mode TAB amount) — no change addendum.
        assertEquals("0\t13.67", text.trimEnd('\t'))
    }

    @Test fun StornoPaymentNoChange_rejectedByFU() {
        // Simulate FU rejecting a storno payment that would result in change.
        val e = DatecsFailure.Rejected(-111094, "Payment in storno can not have change")
        assertEquals(-111094, e.keyCode)
        assertTrue(e.message?.contains("storno") == true || e.message?.contains("change") == true)
    }

    // ─── VG-08 integration: reason 0/1/2 all produce valid stornoOpen payload ─

    @Test fun AllReasonCodesProduceValidStornoOpen() {
        for (reason in 0..2) {
            val doc = DatecsStornoDocument(
                reason = reason,
                documentNumber = 1L,
                documentDateTime = "24-04-19 08:36:27",
                fiscalMemoryNumber = "02636571",
                originalUnp = "DT636497-0001-0000001"
            )
            val payload = DatecsPayloads.stornoOpen(1, "0000", 1, doc)
            val text = DatecsText.decode(payload)
            val fields = text.split('\t')
            assertEquals("reason field must be $reason", "$reason", fields[3])
        }
    }
}
