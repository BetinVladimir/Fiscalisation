package com.beeloy.fiscal.bluecash

import java.math.BigInteger
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class CanonicalJsonTest {
  @Test fun goUtf8OrderAndHtmlEscaping() {
    val map = JSONObject().put("\ud800\udc00", "<>&\u2028\u2029").put("\ue000", "x")
    assertEquals("{\"\ue000\":\"x\",\"\ud800\udc00\":\"\\u003c\\u003e\\u0026\\u2028\\u2029\"}", CanonicalJson.encode(map))
  }
  @Test fun shortestFloatAndExactLong() {
    assertEquals("1e+23", CanonicalJson.encode(1e23))
    assertEquals("5e-324", CanonicalJson.encode(Double.MIN_VALUE))
    assertEquals("0.000001", CanonicalJson.encode(1e-6))
    assertEquals("-0", CanonicalJson.encode(-0.0))
    assertEquals("9223372036854775807", CanonicalJson.encode(Long.MAX_VALUE))
    assertEquals("-9223372036854775808", CanonicalJson.encode(Long.MIN_VALUE))
  }
  @Test fun malformedNumbersCannotEnterSignedWire() {
    assertTrue(runCatching { CanonicalJson.encode(Double.NaN) }.isFailure)
    assertTrue(runCatching { CanonicalJson.int64(1.5) }.isFailure)
    assertTrue(runCatching { CanonicalJson.int64(BigInteger("9223372036854775808")) }.isFailure)
  }
}
