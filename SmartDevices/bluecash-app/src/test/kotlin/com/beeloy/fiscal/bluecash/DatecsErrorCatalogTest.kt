package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

class DatecsErrorCatalogTest {
    @Test
    fun `catalog has at least 13 entries matching spec section 7 row count`() {
        assertTrue(
            "Expected >= 13 entries, got ${DatecsErrorCatalog.entries.size}",
            DatecsErrorCatalog.entries.size >= 13
        )
    }

    @Test
    fun `lookup returns entry for known error code`() {
        val entry = DatecsErrorCatalog.lookup(-100000)
        assertNotNull("Should find -100000", entry)
        assertTrue("Message should not be empty", entry!!.message.isNotEmpty())
    }

    @Test
    fun `all entries have negative codes`() {
        DatecsErrorCatalog.entries.forEach { e ->
            assertTrue("Code ${e.code} should be negative", e.code < 0)
        }
    }

    @Test
    fun `ok parses negative error code category`() {
        val entry = DatecsErrorCatalog.lookup(-100041)
        assertNotNull("Should find -100041 in catalog", entry)
        assertEquals(DatecsErrorCatalog.ErrorCategory.PAPER, entry!!.category)
    }

    @Test
    fun `fiscal errors have fiscal category`() {
        val entry = DatecsErrorCatalog.lookup(-100010)
        assertNotNull("Should find -100010", entry)
        assertEquals(DatecsErrorCatalog.ErrorCategory.FISCAL, entry!!.category)
    }

    @Test
    fun `unknown code returns null`() {
        assertNull("Unknown code should return null", DatecsErrorCatalog.lookup(0))
        assertNull("Unknown code should return null", DatecsErrorCatalog.lookup(-999999))
    }
}
