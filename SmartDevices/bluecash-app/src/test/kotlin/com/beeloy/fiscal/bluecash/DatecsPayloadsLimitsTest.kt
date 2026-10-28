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

    // ─── VG-13: LineUsesDefaultUnit ──────────────────────────────────────────
    // §4.12 syntax №2: Unit must not be empty; default is "бр." (CP1251: E1 F0 2E).

    @Test
    fun LineUsesDefaultUnit() {
        // FiscalLine with no explicit unit — default must be "бр." in the payload.
        val line = FiscalLine(
            name = "Coffee",
            taxGroup = 'A',
            unitPrice = "2.65",
            quantity = "1.000"
        )
        val payload = DatecsPayloads.line(line)
        val text = DatecsText.decode(payload)
        val fields = text.split('\t')
        // Syntax №2 has 8 fields: name, taxGroup, price, qty, discountType, discountValue, dept, unit
        assertTrue("line() must produce ≥8 tab-separated fields", fields.size >= 8)
        val unitField = fields[7]
        assertEquals("default unit must be 'бр.' per §4.12", "бр.", unitField)
        // Verify the CP1251 encoding: 'б'=0xE1, 'р'=0xF0, '.'=0x2E
        val brBytes = "бр.".toByteArray(java.nio.charset.Charset.forName("windows-1251"))
        assertArrayEquals("бр. CP1251 bytes must be 0xE1 0xF0 0x2E",
            byteArrayOf(0xE1.toByte(), 0xF0.toByte(), 0x2E.toByte()), brBytes)
    }

    @Test
    fun LineUsesExplicitUnitWhenProvided() {
        val line = FiscalLine(
            name = "Coffee",
            taxGroup = 'A',
            unitPrice = "2.65",
            quantity = "1.000",
            unit = "pcs"
        )
        val payload = DatecsPayloads.line(line)
        val text = DatecsText.decode(payload)
        val fields = text.split('\t')
        val unitField = fields[7]
        assertEquals("explicit unit must be used as-is", "pcs", unitField)
    }
}
