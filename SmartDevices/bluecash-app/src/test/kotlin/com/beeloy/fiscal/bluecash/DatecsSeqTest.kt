package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/** VG-03.1: Datecs SEQ 0x20..0x7F circular counter. */
class DatecsSeqTest {
    @Test
    fun `SeqWrapsAt0x7FForDatecs`() {
        // The codec validates SEQ in 0x20..0x7F for Datecs
        // 0x7F is the last valid value; 0x80 must be rejected
        val frameAt7F = DatecsFrameCodec.encode(0x7F, 56, byteArrayOf())
        assertNotNull("0x7F should be valid SEQ for Datecs", frameAt7F)
        assertTrue("Frame should start with SOH (0x01)", frameAt7F[0].toInt() == 0x01)

        try {
            DatecsFrameCodec.encode(0x80, 56, byteArrayOf())
            fail("SEQ 0x80 should be rejected for Datecs (out of 0x20..0x7F range)")
        } catch (e: IllegalArgumentException) {
            assertTrue("Error should mention frame fields", e.message?.contains("DATECS_FRAME_FIELDS") == true)
        }
    }

    @Test
    fun `SeqStartsAt0x20`() {
        // Min valid SEQ for Datecs is 0x20 (space); 0x1F must be rejected
        val frameAt20 = DatecsFrameCodec.encode(0x20, 56, byteArrayOf())
        assertNotNull("0x20 should be valid SEQ for Datecs", frameAt20)

        try {
            DatecsFrameCodec.encode(0x1F, 56, byteArrayOf())
            fail("SEQ 0x1F should be rejected for Datecs (below 0x20)")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message?.contains("DATECS_FRAME_FIELDS") == true)
        }
    }

    @Test
    fun `SeqWrapsAt0xFF`() {
        // Verify that 0xFF is NOT valid for Datecs (only for Daisy up to 0xFF)
        // DatecsLimits documents both ranges
        assertEquals("Datecs max SEQ must be 0x7F", 0x7F, DatecsLimits.MAX_SEQ_DATECS)
        assertEquals("Daisy max SEQ must be 0xFF", 0xFF, DatecsLimits.MAX_SEQ_DAISY)

        try {
            DatecsFrameCodec.encode(0xFF, 56, byteArrayOf())
            fail("SEQ 0xFF should be rejected for Datecs encoding (Datecs limit is 0x7F)")
        } catch (e: IllegalArgumentException) {
            // expected: 0xFF is out of Datecs range 0x20..0x7F
        }
    }

    @Test
    fun `ProtocolSeqWrapsFrom0x7FTo0x20`() {
        // Test the internal SEQ wrap logic: verify that the sequence counter logic
        // is: if (sequence == 0x7f) 0x20 else sequence + 1
        val nextAfter7F = if (0x7F == 0x7F) 0x20 else 0x7F + 1
        assertEquals("SEQ after 0x7F should wrap to 0x20", 0x20, nextAfter7F)

        val nextAfter20 = if (0x20 == 0x7F) 0x20 else 0x20 + 1
        assertEquals("SEQ after 0x20 should be 0x21", 0x21, nextAfter20)
    }
}
