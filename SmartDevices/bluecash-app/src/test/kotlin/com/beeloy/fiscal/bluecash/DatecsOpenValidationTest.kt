package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/** VG-06.2: open() strict UNP regex and percentage discount 0.01-100.00. */
class DatecsOpenValidationTest {
    @Test
    fun `OpenRequiresSpecUnpFormat`() {
        // Valid BG UNP: 2 uppercase + 6 digits + hyphen + 4 alphanumeric + hyphen + 7 digits
        val bytes = DatecsPayloads.open(1, "0000", "DY000600-A001-0000001", 1)
        assertNotNull("Valid UNP should succeed", bytes)

        try {
            DatecsPayloads.open(1, "0000", "invalid-unp", 1)
            fail("Should reject invalid UNP format")
        } catch (e: IllegalArgumentException) {
            // expected
        }

        try {
            DatecsPayloads.open(1, "0000", "TOOOO SHORT", 1)
            fail("Should reject UNP not matching regex")
        } catch (e: IllegalArgumentException) {
            // expected
        }
    }

    @Test
    fun `PercentageDiscountInRange`() {
        // discountType 2 = percentage subtraction; valid range 0.01-100.00
        val lineOk = FiscalLine(
            name = "Coffee",
            taxGroup = 'B',
            unitPrice = "2.65",
            quantity = "1.000",
            discountType = 2,
            discountValue = "10.00",
        )
        assertNotNull(DatecsPayloads.line(lineOk))
    }

    @Test
    fun `PercentageDiscountZeroRejected`() {
        val line = FiscalLine(
            name = "Coffee",
            taxGroup = 'B',
            unitPrice = "2.65",
            quantity = "1.000",
            discountType = 2,
            discountValue = "0.00",
        )
        try {
            DatecsPayloads.line(line)
            fail("0.00% discount should be rejected")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message?.contains("percent") == true)
        }
    }

    @Test
    fun `PercentageDiscountAbove100Rejected`() {
        val line = FiscalLine(
            name = "Coffee",
            taxGroup = 'B',
            unitPrice = "2.65",
            quantity = "1.000",
            discountType = 2,
            discountValue = "100.01",
        )
        try {
            DatecsPayloads.line(line)
            fail("100.01% discount should be rejected")
        } catch (e: IllegalArgumentException) {
            // expected
        }
    }

    @Test
    fun `PercentageDiscountAt100Accepted`() {
        val line = FiscalLine(
            name = "Coffee",
            taxGroup = 'B',
            unitPrice = "2.65",
            quantity = "1.000",
            discountType = 2,
            discountValue = "100.00",
        )
        assertNotNull("100% discount should be accepted", DatecsPayloads.line(line))
    }
}
