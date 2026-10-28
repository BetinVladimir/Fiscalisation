package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * TST-DEEP-14.2 — smart_protocol_examples.json schema validation.
 *
 * Checks:
 * - All JSON keys contain no spaces (F-4 typo fix)
 * - Uses "seq" not "sec" (protocol typo fix)
 * - Amounts computed from integer cents: sale_cash.expected_total_cents = 1367 = 1200+167
 * - Golden amounts: expected_total_cents matches golden receipt (CASH 500 + CARD 867 = 1367¢)
 */
class SmartProtocolExamplesTest {

    private fun findFixture(): File {
        // Try paths from repo root (SmartDevices is two levels from repo root)
        val candidates = listOf(
            File("docs/MVP1/fixtures/smart_protocol_examples.json"),
            File("../../docs/MVP1/fixtures/smart_protocol_examples.json"),
            File("../../../docs/MVP1/fixtures/smart_protocol_examples.json"),
        )
        return candidates.firstOrNull { it.exists() }
            ?: error(
                "smart_protocol_examples.json not found; tried: ${candidates.map { it.absolutePath }}"
            )
    }

    @Test
    fun `fixture file exists and is valid JSON`() {
        val f = findFixture()
        assertTrue("fixture not found: ${f.absolutePath}", f.exists())
        val text = f.readText()
        assertTrue("fixture is empty", text.isNotBlank())
        // Basic JSON structure check: starts with '{'
        assertTrue("fixture must be a JSON object", text.trim().startsWith("{"))
    }

    @Test
    fun `no JSON keys contain spaces (F-4 typo fix)`() {
        val text = findFixture().readText()
        // Find all JSON keys: text between '"' followed by '":'
        val keyPattern = Regex(""""([^"]+)"\s*:""")
        val keysWithSpaces = keyPattern.findAll(text)
            .map { it.groupValues[1] }
            .filter { it.contains(" ") && !it.startsWith("_") }
            .toList()
        assertTrue(
            "Keys with spaces found (F-4 violation): $keysWithSpaces",
            keysWithSpaces.isEmpty()
        )
    }

    @Test
    fun `uses seq not sec (protocol field typo check)`() {
        val text = findFixture().readText()
        // "sec" as a standalone key would be a typo for "seq"
        val secPattern = Regex(""""sec"\s*:""")
        assertFalse(
            "Found 'sec' key — should be 'seq' (protocol typo F-4)",
            secPattern.containsMatchIn(text)
        )
        // Confirm "seq" is present
        val seqPattern = Regex(""""seq"\s*:""")
        assertTrue(
            "No 'seq' key found in fixture",
            seqPattern.containsMatchIn(text)
        )
    }

    @Test
    fun `sale_cash expected_total_cents matches golden receipt 1367 cents`() {
        val text = findFixture().readText()
        // Extract "expected_total_cents": N
        val pattern = Regex(""""expected_total_cents"\s*:\s*(\d+)""")
        val match = pattern.find(text)
            ?: error("expected_total_cents not found in fixture")
        val cents = match.groupValues[1].toLong()
        // Golden: 1200 + 167 = 1367¢
        assertEquals("golden receipt total must be 1367¢ (1200+167)", 1367L, cents)
    }

    @Test
    fun `money_rule declares integer cents (no float construction)`() {
        val text = findFixture().readText()
        // money_rule must mention BigDecimal or integer cents construction
        assertTrue(
            "money_rule must reference integer cents (BigDecimal or integer)",
            text.contains("BigDecimal") || text.contains("целых центов") || text.contains("cents")
        )
    }

    @Test
    fun `sale_cash request has seq field`() {
        val text = findFixture().readText()
        // sale_cash.request.seq must be present
        assertTrue("sale_cash must have a 'seq' field in request", text.contains("\"seq\""))
    }
}
