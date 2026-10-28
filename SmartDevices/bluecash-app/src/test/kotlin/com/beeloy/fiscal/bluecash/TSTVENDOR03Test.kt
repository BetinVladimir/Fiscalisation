package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Vendor spec: Datecs PM §4.12 — fiscal text is encoded as CP1251 (windows-1251).
 * Invariant 7: all FU text is CP1251; unsupported chars must be rejected before I/O.
 * Length limit is 72 bytes in CP1251 (=72 Cyrillic chars since CP1251 is single-byte).
 */
class TSTVENDOR03Test {

    // PM §4.12 example: "Топено сирене" = 0xD2 0xEE 0xEF 0xE5 0xED 0xEE 0x20 0xF1 0xE8 0xF0 0xE5 0xED 0xE5
    @Test fun `topenо sirene encodes to vendor PM section 4-12 bytes`() {
        val name = "Топено сирене"
        val encoded = DatecsText.encode(name)
        val expected = byteArrayOf(
            0xD2.toByte(), 0xEE.toByte(), 0xEF.toByte(), 0xE5.toByte(),
            0xED.toByte(), 0xEE.toByte(), 0x20.toByte(), 0xF1.toByte(),
            0xE8.toByte(), 0xF0.toByte(), 0xE5.toByte(), 0xED.toByte(), 0xE5.toByte()
        )
        assertArrayEquals("PM §4.12 literal bytes must match", expected, encoded)
    }

    @Test fun `emoji is rejected before IO as DATECS_TEXT_UNMAPPABLE`() {
        try {
            DatecsText.encode("😀")
            error("expected IllegalArgumentException for unsupported char")
        } catch (e: IllegalArgumentException) {
            assertTrue(
                "error message must contain DATECS_TEXT_UNMAPPABLE, got: ${e.message}",
                e.message?.contains("DATECS_TEXT_UNMAPPABLE") == true
            )
        }
    }

    @Test fun `72 cyrillic chars passes line name length limit`() {
        val name72 = "А".repeat(72) // 72 Cyrillic А = 72 CP1251 bytes
        val line = FiscalLine(
            name = name72,
            taxGroup = 'A',
            unitPrice = "12.00",
            quantity = "1.000",
            discountType = 0,
            discountValue = "",
            department = 0
        )
        // Must not throw
        DatecsPayloads.line(line)
    }

    @Test fun `73 cyrillic chars fails line name length limit`() {
        val name73 = "А".repeat(73) // 73 Cyrillic А = 73 CP1251 bytes, exceeds limit
        val line = FiscalLine(
            name = name73,
            taxGroup = 'A',
            unitPrice = "12.00",
            quantity = "1.000",
            discountType = 0,
            discountValue = "",
            department = 0
        )
        try {
            DatecsPayloads.line(line)
            error("expected IllegalArgumentException for name > 72 bytes")
        } catch (e: IllegalArgumentException) {
            // expected
        }
    }

    @Test fun `decode roundtrips cyrillic through CP1251`() {
        val original = "Топено сирене"
        val encoded = DatecsText.encode(original)
        val decoded = DatecsText.decode(encoded)
        assertEquals("Roundtrip must preserve Cyrillic text", original, decoded)
    }
}
