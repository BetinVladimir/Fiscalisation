package com.beeloy.fiscal.bluecash

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.time.LocalDate
import org.junit.Assert.*
import org.junit.Test

class DatecsPinpadProtocolTest {
  private fun packet(event: Int, error: Int, payload: ByteArray): ByteArray {
    val out =
      mutableListOf(
        0x3e.toByte(),
        event.toByte(),
        error.toByte(),
        (payload.size shr 8).toByte(),
        payload.size.toByte()
      )
    out += payload.toList()
    out += out.fold(0) { a, b -> a xor (b.toInt() and 255) }.toByte()
    return out.toByteArray()
  }
  @Test
  fun `ber tlv vectors preserve Datecs tags and amounts`() {
    val raw =
      DatecsBerTlv.encode(
        listOf(BerTlv(0x9A, byteArrayOf(0x26, 0x08, 0x11)), BerTlv(0x81, byteArrayOf(0, 0, 1, 9)))
      )
    val tags = DatecsBerTlv.decode(raw)
    assertEquals(listOf(0x9AL, 0x81L), tags.map { it.tag })
    assertEquals(265, DatecsBerTlv.integer(tags[1].value))
  }
  @Test
  fun `purchase sends borica command and parses approved event`() {
    val tlv =
      DatecsBerTlv.encode(
        listOf(
          BerTlv(0xDF05, byteArrayOf(0)),
          BerTlv(0xDF06, byteArrayOf(0)),
          BerTlv(0xDF09, byteArrayOf(0)),
          BerTlv(0xDF07, "RRN1".toByteArray()),
          BerTlv(0xDF08, "AUTH".toByteArray())
        )
      )
    val input =
      ByteArrayInputStream(
        packet(0, 0, byteArrayOf()) +
          packet(0, 0, byteArrayOf()) +
          packet(0, 0, byteArrayOf()) +
          packet(14, 0, byteArrayOf(1) + tlv)
      )
    val output = ByteArrayOutputStream()
    val result =
      BoricaPinpadCodec(today = { LocalDate.of(2026, 8, 11) }).purchase(265, "op", input, output)
    assertEquals("true", result["approved"])
    assertTrue(
      output.toByteArray().toList().windowed(2).any {
        it[0] == 0x3e.toByte() && it[1] == 61.toByte()
      }
    )
  }
  @Test
  fun `approved original survives codec restart and can be reversed`() {
    val store = MemoryCardTransactionStore()
    val approved =
      DatecsBerTlv.encode(
        listOf(
          BerTlv(0xDF05, byteArrayOf(0)),
          BerTlv(0xDF06, byteArrayOf(0)),
          BerTlv(0xDF09, byteArrayOf(0)),
          BerTlv(0xDF07, "RRN1".toByteArray()),
          BerTlv(0xDF08, "AUTH".toByteArray())
        )
      )
    val purchaseIn =
      ByteArrayInputStream(
        packet(0, 0, byteArrayOf()) +
          packet(0, 0, byteArrayOf()) +
          packet(0, 0, byteArrayOf()) +
          packet(14, 0, byteArrayOf(1) + approved)
      )
    BoricaPinpadCodec({ LocalDate.of(2026, 8, 11) }, store)
      .purchase(265, "op", purchaseIn, ByteArrayOutputStream())
    val reverseIn =
      ByteArrayInputStream(
        packet(0, 0, byteArrayOf()) +
          packet(0, 0, byteArrayOf()) +
          packet(14, 0, byteArrayOf(1) + approved)
      )
    val result =
      BoricaPinpadCodec({ LocalDate.of(2026, 8, 12) }, store)
        .reverse("op", reverseIn, ByteArrayOutputStream())
    assertEquals("true", result["approved"])
    assertEquals("REVERSED", store.original("op")?.state)
  }
  @Test
  fun `wire rejects corrupt bcc`() {
    val bytes = packet(0, 0, byteArrayOf())
    bytes[bytes.lastIndex] = (bytes.last().toInt() xor 1).toByte()
    assertThrows(IllegalArgumentException::class.java) {
      DatecsPinpadWire(ByteArrayInputStream(bytes), ByteArrayOutputStream()).command(61, 3)
    }
  }
}
