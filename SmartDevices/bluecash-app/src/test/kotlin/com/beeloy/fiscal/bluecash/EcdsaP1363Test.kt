package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

class EcdsaP1363Test {
  @Test
  fun `canonical DER becomes fixed P1363`() {
    val der = byteArrayOf(0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02)
    val out = EcdsaP1363.fromDer(der)
    assertEquals(64, out.size)
    assertEquals(1, out[31].toInt())
    assertEquals(2, out[63].toInt())
  }
  @Test
  fun `non canonical DER is rejected`() {
    for (v in
      listOf(byteArrayOf(), byteArrayOf(0x30, 0x06, 0x02, 0x02, 0, 1, 0x02, 0x01, 2))) runCatching {
        EcdsaP1363.fromDer(v)
      }
      .onSuccess { fail("accepted") }
      .onFailure { assertTrue(it is IllegalArgumentException) }
  }
}
