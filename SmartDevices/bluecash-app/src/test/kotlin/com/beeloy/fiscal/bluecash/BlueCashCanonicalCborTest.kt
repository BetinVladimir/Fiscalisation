package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class BlueCashCanonicalCborTest {
  @Test
  fun `canonical RFC map vector is stable`() {
    val encoded =
      BlueCashCanonicalCbor.encode(mapOf("aa" to 2, "b" to 1, "a" to listOf(true, null, "x")))
    assertArrayEquals(
      byteArrayOf(
        0xa3.toByte(),
        0x61,
        0x61,
        0x83.toByte(),
        0xf5.toByte(),
        0xf6.toByte(),
        0x61,
        0x78,
        0x61,
        0x62,
        0x01,
        0x62,
        0x61,
        0x61,
        0x02
      ),
      encoded
    )
    val decoded = BlueCashCanonicalCbor.decodeMap(encoded)
    assertEquals(1L, decoded["b"])
  }
  @Test
  fun `non canonical integer duplicate and indefinite containers are rejected`() {
    assertThrows(IllegalArgumentException::class.java) {
      BlueCashCanonicalCbor.decode(byteArrayOf(0x18, 0x01))
    }
    assertThrows(IllegalArgumentException::class.java) {
      BlueCashCanonicalCbor.decode(byteArrayOf(0xa2.toByte(), 0x61, 0x61, 0x01, 0x61, 0x61, 0x02))
    }
    assertThrows(IllegalArgumentException::class.java) {
      BlueCashCanonicalCbor.decode(byteArrayOf(0x9f.toByte(), 0xff.toByte()))
    }
  }
}
