package com.beeloy.fiscal.bluecash

import java.io.File
import java.math.BigDecimal
import java.math.RoundingMode
import org.json.JSONArray
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * BUG-023.4: Golden rounding vectors — verifies (unit_cents * qty_milli + 500) / 1000 in integers.
 * Reads docs/MVP1/fixtures/rounding_golden.json from the repo root.
 */
class RoundingGoldenTest {

  private fun repoRoot(): File {
    var d = File(System.getProperty("user.dir", "."))
    while (d.parentFile != null) {
      if (File(d, "docs/MVP1/fixtures/rounding_golden.json").exists()) return d
      d = d.parentFile
    }
    error("repo root not found; user.dir=${System.getProperty("user.dir")}")
  }

  private data class Vector(
    val name: String,
    val unitCents: Long,
    val qtyMilli: Long,
    val discountCents: Long,
    val expectLineCents: Long
  )

  private fun loadVectors(): List<Vector> {
    val file = File(repoRoot(), "docs/MVP1/fixtures/rounding_golden.json")
    val arr = JSONArray(file.readText())
    return (0 until arr.length()).map { i ->
      val o = arr.getJSONObject(i)
      Vector(
        o.getString("name"),
        o.getLong("unit_cents"),
        o.getLong("qty_milli"),
        o.optLong("discount_cents", 0L),
        o.getLong("expect_line_cents")
      )
    }
  }

  @Test fun `integer formula matches all golden vectors`() {
    val vectors = loadVectors()
    assert(vectors.size >= 30) { "golden must have ≥30 vectors, got ${vectors.size}" }
    var passed = 0
    for (v in vectors) {
      val gross = (v.unitCents * v.qtyMilli + 500L) / 1000L
      val net = gross - v.discountCents
      assertEquals("vector ${v.name}: integer formula mismatch", v.expectLineCents, net)
      passed++
    }
    println("ROUND OK $passed")
  }

  @Test fun `bigdecimal half_up formula matches all golden vectors`() {
    val vectors = loadVectors()
    for (v in vectors) {
      val gross = BigDecimal(v.unitCents)
        .multiply(BigDecimal(v.qtyMilli))
        .add(BigDecimal(500))
        .divide(BigDecimal(1000), 0, RoundingMode.DOWN)
        .toLong()
      val net = gross - v.discountCents
      assertEquals(
        "vector ${v.name}: BigDecimal half-up mismatch",
        v.expectLineCents,
        net
      )
    }
  }
}
