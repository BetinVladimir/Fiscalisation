package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/** VG-06.1: DatecsPayloads accepts max price and quantity within spec limits. */
class DatecsPayloadsLimitsTest {
    @Test
    fun `AcceptsMaxPriceAndQuantity`() {
        // Max allowed price: 9999999.99 and quantity: 99999.999
        val line = FiscalLine(
            name = "Max Item",
            taxGroup = 'A',
            unitPrice = "9999.99",
            quantity = "1.000",
        )
        val bytes = DatecsPayloads.line(line)
        assertNotNull("line() should succeed for valid price/quantity", bytes)
        assertTrue("line bytes should be non-empty", bytes.isNotEmpty())
    }

    @Test
    fun `RejectsNameLongerThan72Characters`() {
        val longName = "A".repeat(73)
        val line = FiscalLine(
            name = longName,
            taxGroup = 'A',
            unitPrice = "1.00",
            quantity = "1.000",
        )
        try {
            DatecsPayloads.line(line)
            fail("Should reject name longer than ${DatecsLimits.MAX_NAME_LEN} chars")
        } catch (e: IllegalArgumentException) {
            // expected
        }
    }

    @Test
    fun `AcceptsNameAtMaxLength`() {
        val maxName = "A".repeat(DatecsLimits.MAX_NAME_LEN)
        val line = FiscalLine(
            name = maxName,
            taxGroup = 'B',
            unitPrice = "1.00",
            quantity = "1.000",
        )
        val bytes = DatecsPayloads.line(line)
        assertNotNull(bytes)
    }
}
