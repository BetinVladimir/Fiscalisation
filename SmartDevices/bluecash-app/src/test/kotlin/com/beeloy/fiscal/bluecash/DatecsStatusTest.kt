package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/** VG-02.1: DatecsStatus parses 8 status bytes, fatal/warning flags. */
class DatecsStatusTest {
    private fun status(vararg bytes: Int): DatecsStatus =
        DatecsStatus.parse(ByteArray(8) { i -> if (i < bytes.size) bytes[i].toByte() else 0 })

    @Test
    fun `StatusDecodesAllBytesFromSpecExamples`() {
        // All-zero = OK: no fatal, no warning
        val ok = status(0x00)
        assertFalse("all-zero: fatal should be false", ok.fatal)
        assertFalse("all-zero: warning should be false", ok.warning)

        // Byte0 bit5 set = error (fatal)
        val withError = status(0x20)
        assertTrue("bit5 in byte0: fatal should be true", withError.fatal)

        // Byte2 bit5 set = error in second status byte (fatal)
        val withError2 = status(0x00, 0x00, 0x20)
        assertTrue("bit5 in byte2: fatal should be true", withError2.fatal)

        // Byte0 bit1 set = near end of paper (warning, not fatal)
        val nearPaper = status(0x02)
        assertFalse("near-paper: fatal should be false", nearPaper.fatal)
        assertTrue("near-paper: warning should be true", nearPaper.warning)

        // Byte0 bit4 set = fiscal memory almost full (warning, not fatal)
        val memFull = status(0x10)
        assertFalse("mem-full bit4: fatal should be false", memFull.fatal)
        assertTrue("mem-full bit4: warning should be true", memFull.warning)

        // Fatal overrides warning
        val both = status(0x22) // bit5=error + bit1=near-paper
        assertTrue("both: fatal should be true", both.fatal)
        assertTrue("both: warning should be true", both.warning)
    }

    @Test
    fun `StatusRequires8Bytes`() {
        try {
            DatecsStatus(byteArrayOf(0x00, 0x00))
            fail("Should have thrown for wrong size")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message?.contains("8") == true)
        }
    }
}
