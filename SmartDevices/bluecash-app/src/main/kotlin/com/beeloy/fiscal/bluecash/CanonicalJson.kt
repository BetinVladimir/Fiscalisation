package com.beeloy.fiscal.bluecash

import java.math.BigDecimal
import java.math.BigInteger
import java.math.MathContext
import java.math.RoundingMode
import org.json.JSONArray
import org.json.JSONObject

/** Go encoding/json map encoding; typed Go structures must retain their declared field order. */
object CanonicalJson {
  private val utf8Order = Comparator<String> { a, b ->
    val x = a.toByteArray(Charsets.UTF_8)
    val y = b.toByteArray(Charsets.UTF_8)
    var result = 0
    var i = 0
    while (result == 0 && i < minOf(x.size, y.size)) {
      result = (x[i].toInt() and 255).compareTo(y[i].toInt() and 255)
      i++
    }
    if (result == 0) x.size.compareTo(y.size) else result
  }

  fun encode(value: Any?): String = when (value) {
    null, JSONObject.NULL -> "null"
    is JSONObject -> value.keys().asSequence().toList().sortedWith(utf8Order)
      .joinToString(",", "{", "}") { quote(it) + ":" + encode(value.get(it)) }
    is JSONArray -> (0 until value.length()).joinToString(",", "[", "]") { encode(value.get(it)) }
    is String -> quote(value)
    is Boolean -> value.toString()
    is Byte, is Short, is Int, is Long, is BigInteger -> value.toString()
    is Double -> floating(value, false)
    is Float -> floating(value.toDouble(), true)
    // org.json JVM parses fractional JSON tokens as BigDecimal; Go unmarshal
    // into map[string]any parses these tokens as float64.
    is BigDecimal -> floating(value.toDouble(), false)
    else -> error("GO_JSON_UNSUPPORTED_TYPE")
  }

  fun quote(value: String): String = buildString {
    append('"')
    var i = 0
    while (i < value.length) {
      val c = value[i]
      when (c) {
        '"' -> append("\\\"")
        '\\' -> append("\\\\")
        '\b' -> append("\\b")
        '\u000c' -> append("\\f")
        '\n' -> append("\\n")
        '\r' -> append("\\r")
        '\t' -> append("\\t")
        '<', '>', '&', '\u2028', '\u2029' -> append("\\u" + c.code.toString(16).padStart(4, '0'))
        else -> when {
          c.code < 32 -> append("\\u" + c.code.toString(16).padStart(4, '0'))
          c.isHighSurrogate() && i + 1 < value.length && value[i + 1].isLowSurrogate() -> {
            append(c); append(value[++i])
          }
          c.isSurrogate() -> append("\\ufffd")
          else -> append(c)
        }
      }
      i++
    }
    append('"')
  }

  /** Exact integer access for typed signed int64 fields; never truncate Double or overflow. */
  fun int64(value: Any?): Long = when (value) {
    is Byte, is Short, is Int, is Long -> (value as Number).toLong()
    is BigInteger -> { require(value.bitLength() <= 63) { "GO_JSON_INT64_OVERFLOW" }; value.toLong() }
    else -> error("GO_JSON_INT64_REQUIRED")
  }

  private fun floating(value: Double, float32: Boolean): String {
    require(value.isFinite()) { "GO_JSON_NONFINITE" }
    if (value == 0.0) return if (java.lang.Double.doubleToRawLongBits(value) < 0) "-0" else "0"
    val negative = value < 0
    val magnitude = kotlin.math.abs(value)
    val exact = BigDecimal(magnitude)
    val bits = if (float32) java.lang.Float.floatToRawIntBits(magnitude.toFloat()).toLong()
      else java.lang.Double.doubleToRawLongBits(magnitude)
    val mantissaBits = if (float32) 23 else 52
    val exponentMask = if (float32) 255L else 2047L
    val exponent = (bits ushr mantissaBits) and exponentMask
    // Pinned Go 1.26 Dragonbox Algorithm 5.6 selects the upper midpoint in
    // normal powers-of-two shorter intervals, except binary exponent -77.
    val upperTie = exponent != 0L && bits and ((1L shl mantissaBits) - 1) == 0L &&
      exponent - (if (float32) 127 else 1023) - mantissaBits != -77L
    var shortest: BigDecimal? = null
    // Search the decimal grids directly using exact binary->decimal conversion.
    // Java/Android Double.toString differ across runtimes (notably 1e23 and
    // subnormals). Adjacent rounded candidates cover the asymmetric interval at
    // powers of two; nearest candidate wins, with an even last digit on ties.
    for (precision in 1..if (float32) 9 else 17) {
      var best: BigDecimal? = null
      for (mode in arrayOf(RoundingMode.HALF_EVEN, RoundingMode.FLOOR, RoundingMode.CEILING)) {
        val candidate = exact.round(MathContext(precision, mode)).stripTrailingZeros()
        val roundtrips = if (float32) candidate.toFloat() == magnitude.toFloat()
          else java.lang.Double.doubleToRawLongBits(candidate.toDouble()) == java.lang.Double.doubleToRawLongBits(magnitude)
        if (!roundtrips) continue
        val incumbent = best
        if (incumbent == null || candidate.subtract(exact).abs() < incumbent.subtract(exact).abs() ||
          (candidate.subtract(exact).abs() == incumbent.subtract(exact).abs() &&
            (if (upperTie) candidate > incumbent else !candidate.unscaledValue().testBit(0)))) best = candidate
      }
      if (best != null) { shortest = best; break }
    }
    val decimal = checkNotNull(shortest)
    // encoding/json uses ES6 exponent cutoffs; float32 cutoff comparison also
    // takes place in float32 to match the Go encoder.
    val exponential = if (float32) magnitude.toFloat() < 1e-6f || magnitude.toFloat() >= 1e21f
      else magnitude < 1e-6 || magnitude >= 1e21
    val body = if (!exponential) decimal.toPlainString() else {
      val digits = decimal.unscaledValue().abs().toString()
      val exponent = digits.length - decimal.scale() - 1
      digits.take(1) + (if (digits.length > 1) "." + digits.drop(1) else "") +
        "e" + (if (exponent >= 0) "+" else "") + exponent
    }
    return (if (negative) "-" else "") + body
  }
}
