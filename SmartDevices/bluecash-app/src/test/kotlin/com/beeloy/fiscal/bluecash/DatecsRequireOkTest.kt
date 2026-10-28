package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/** VG-02.3: requireOk(expectedFields) — empty data = failure, ErrorCode int. */
class DatecsRequireOkTest {
    private fun response(data: String, fields: Int = 1) =
        DatecsResponse(0x21, 56, DatecsText.encode(data), ByteArray(8))

    @Test
    fun `OkRejectsEmptyData`() {
        val r = DatecsResponse(0x21, 56, byteArrayOf(), ByteArray(8))
        try {
            r.requireOk(1)
            fail("requireOk should throw on empty data")
        } catch (e: DatecsFailure.Rejected) {
            assertTrue("empty data error code must be int 0", e.keyCode == 0)
        }
    }

    @Test
    fun `OkPassesOnZeroErrorCode`() {
        val r = response("0\tsome-value\tmore")
        assertNotNull(r.requireOk(1))
    }

    @Test
    fun `OkRejectsNonZeroErrorCode`() {
        val r = response("5\tsome-error")
        try {
            r.requireOk(1)
            fail("requireOk should throw on non-zero error code")
        } catch (e: DatecsFailure.Rejected) {
            assertEquals("error code must be 5", 5, e.keyCode)
        }
    }

    @Test
    fun `OkRejectsTooFewFields`() {
        val r = response("0") // only 1 field
        try {
            r.requireOk(2)
            fail("requireOk should throw when fewer fields than expected")
        } catch (e: DatecsFailure.Rejected) {
            assertTrue(e.message?.contains("MISSING_FIELDS") == true)
        }
    }
}
