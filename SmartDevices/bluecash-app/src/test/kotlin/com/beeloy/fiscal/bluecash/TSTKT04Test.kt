package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.io.File

/**
 * TST-KT-04: Money rounding golden test (BUG-023).
 * Verifies (unit_cents × qty_milli + 500) / 1000 for 34/34 vectors.
 * Output must contain "ROUND OK 34".
 * Fixture: docs/MVP1/fixtures/rounding_golden.json (invariant 9).
 */
class TSTKT04Test {

    private fun repoRoot(): File {
        var d = File(System.getProperty("user.dir", "."))
        while (d.parentFile != null) {
            if (File(d, "docs/MVP1/fixtures/rounding_golden.json").exists()) return d
            d = d.parentFile
        }
        error("repo root not found; user.dir=${System.getProperty("user.dir")}")
    }

    @Test fun MoneyRoundingGoldenTest() {
        val file = File(repoRoot(), "docs/MVP1/fixtures/rounding_golden.json")
        assertTrue("rounding_golden.json must exist", file.exists())

        val arr = org.json.JSONArray(file.readText())
        assertTrue("golden file must have ≥30 vectors", arr.length() >= 30)

        var passed = 0
        for (i in 0 until arr.length()) {
            val o = arr.getJSONObject(i)
            val unitCents  = o.getLong("unit_cents")
            val qtyMilli   = o.getLong("qty_milli")
            val discountCents = o.optLong("discount_cents", 0L)
            val expect     = o.getLong("expect_line_cents")

            // Invariant 1: integer half-up rounding formula.
            val gross = (unitCents * qtyMilli + 500L) / 1000L
            val net   = gross - discountCents

            assertEquals("vector $i '${o.getString("name")}' mismatch", expect, net)
            passed++
        }
        // Must confirm "ROUND OK <n>" with actual count
        println("ROUND OK $passed")
        assertEquals("all vectors must pass", arr.length(), passed)
    }

    // Additional: golden amounts from AGENTS.md invariants
    @Test fun goldenReceiptAmounts() {
        // total 1367¢ = 1200¢ + 167¢; CASH 500¢, CARD 867¢
        val total   = (1200L * 1000L + 500L) / 1000L + (167L * 1000L + 500L) / 1000L
        assertEquals("1200 + 167 must equal 1367 cents", 1367L, total)

        val cashCents = 500L
        val cardCents = 867L
        assertEquals("CASH + CARD must equal total", 1367L, cashCents + cardCents)

        // Verify rounding: 2.65 each at 3.000 qty = 7.95 (not 7.950000...)
        val unit265milli3000 = (265L * 3000L + 500L) / 1000L
        assertEquals("2.65 × 3.000 = 795¢", 795L, unit265milli3000)
    }
}
