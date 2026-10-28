package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.File

/** SMS-08.2: SmartMoney — 34 rounding_golden.json vectors, no double artifacts. */
class SmartMoneyTest {

    @Test fun `centsToJson golden values`() {
        assertEquals("13.67", SmartMoney.centsToJson(1367))
        assertEquals("5.00", SmartMoney.centsToJson(500))
        assertEquals("8.67", SmartMoney.centsToJson(867))
        assertEquals("0.00", SmartMoney.centsToJson(0))
        assertEquals("100.00", SmartMoney.centsToJson(10000))
    }

    @Test fun `qtyMilliToJson golden values`() {
        assertEquals("0.500", SmartMoney.qtyMilliToJson(500))
        assertEquals("1.000", SmartMoney.qtyMilliToJson(1000))
        assertEquals("0.333", SmartMoney.qtyMilliToJson(333))
        assertEquals("0.001", SmartMoney.qtyMilliToJson(1))
    }

    @Test fun `34 rounding_golden vectors pass with half-up integer arithmetic`() {
        // Load vectors from docs/MVP1/fixtures/rounding_golden.json using file path relative to
        // the project root (two levels up from SmartDevices/).
        val fixtureFile = findRoundingGolden()
        val content = fixtureFile.readText()
        // Parse JSON arrays manually to avoid dependencies.
        val vectors = parseRoundingVectors(content)
        assertEquals("Must have exactly 34 vectors", 34, vectors.size)
        var passed = 0
        for (v in vectors) {
            val actual = SmartMoney.lineCents(v.unitCents, v.qtyMilli, v.discountCents)
            assertEquals(
                "Vector ${v.name}: lineCents(${v.unitCents}, ${v.qtyMilli})",
                v.expectLineCents, actual
            )
            passed++
        }
        assertEquals("All 34 vectors must pass", 34, passed)
    }

    private fun findRoundingGolden(): File {
        // Walk up to find docs/MVP1/fixtures/rounding_golden.json
        var dir = File(System.getProperty("user.dir"))
        repeat(5) {
            val candidate = File(dir, "docs/MVP1/fixtures/rounding_golden.json")
            if (candidate.exists()) return candidate
            dir = dir.parentFile ?: return@repeat
        }
        error("rounding_golden.json not found from ${System.getProperty("user.dir")}")
    }

    private data class RoundingVector(
        val name: String,
        val unitCents: Long,
        val qtyMilli: Long,
        val discountCents: Long,
        val expectLineCents: Long,
    )

    private fun parseRoundingVectors(json: String): List<RoundingVector> {
        val result = mutableListOf<RoundingVector>()
        val objectPattern = Regex("""\{[^}]+\}""")
        for (match in objectPattern.findAll(json)) {
            val obj = match.value
            fun field(key: String) = Regex(""""$key"\s*:\s*(-?\d+|"[^"]*")""").find(obj)
                ?.groupValues?.get(1)?.removeSurrounding("\"")
            val name = field("name") ?: continue
            val unitCents = field("unit_cents")?.toLongOrNull() ?: continue
            val qtyMilli = field("qty_milli")?.toLongOrNull() ?: continue
            val discountCents = field("discount_cents")?.toLongOrNull() ?: 0L
            val expect = field("expect_line_cents")?.toLongOrNull() ?: continue
            result += RoundingVector(name, unitCents, qtyMilli, discountCents, expect)
        }
        return result
    }
}
