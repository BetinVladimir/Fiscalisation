package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

class DatecsLimitsTest {
    @Test
    fun `SEQ range starts at 0x20`() {
        assertEquals(0x20, DatecsLimits.MIN_SEQ)
    }

    @Test
    fun `Datecs SEQ ends at 0x7F`() {
        assertEquals(0x7F, DatecsLimits.MAX_SEQ_DATECS)
    }

    @Test
    fun `Daisy SEQ ends at 0xFF`() {
        assertEquals(0xFF, DatecsLimits.MAX_SEQ_DAISY)
    }

    @Test
    fun `LEN offset for Datecs is 0x2A`() {
        assertEquals(0x2A, DatecsLimits.LEN_OFFSET_DATECS)
    }

    @Test
    fun `LEN offset for Daisy is 0x24`() {
        assertEquals(0x24, DatecsLimits.LEN_OFFSET_DAISY)
    }

    @Test
    fun `SEQ range is valid for both brands`() {
        assertTrue("MIN_SEQ must be printable ASCII", DatecsLimits.MIN_SEQ >= 0x20)
        assertTrue("Datecs MAX_SEQ fits in 7-bit ASCII", DatecsLimits.MAX_SEQ_DATECS <= 0x7F)
        assertTrue("Daisy MAX_SEQ fits in byte", DatecsLimits.MAX_SEQ_DAISY <= 0xFF)
        assertTrue("Daisy range wider than Datecs", DatecsLimits.MAX_SEQ_DAISY > DatecsLimits.MAX_SEQ_DATECS)
    }
}
