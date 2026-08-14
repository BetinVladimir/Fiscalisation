package com.beeloy.fiscal.bluecash

import java.util.UUID
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Test

class BlueCashBleFrameTest {
  private val secret = ByteArray(32) { it.toByte() }
  private val sessionId = "ble-session-1"
  private val id = UUID.fromString("550e8400-e29b-41d4-a716-446655440101")
  @Test
  fun `client and edge use compatible directional keys and reject replay`() {
    val client = BlueCashBleFrameSession.client(secret, sessionId)
    val edge = BlueCashBleFrameSession.edge(secret, sessionId)
    val raw = client.seal(id, 0, 1, 0, "hello".toByteArray())
    val opened = edge.open(raw)
    assertEquals(id, opened.messageId)
    assertArrayEquals("hello".toByteArray(), opened.plaintext)
    assertThrows(IllegalArgumentException::class.java) { edge.open(raw) }
    assertArrayEquals(
      "result".toByteArray(),
      client.open(edge.seal(id, 0, 1, 1, "result".toByteArray())).plaintext
    )
  }
  @Test
  fun `tamper is rejected before replay counter advances`() {
    val client = BlueCashBleFrameSession.client(secret, sessionId)
    val edge = BlueCashBleFrameSession.edge(secret, sessionId)
    val raw = client.seal(id, 0, 1, 0, "hello".toByteArray())
    val changed = raw.copyOf().also { it[it.lastIndex] = (it.last().toInt() xor 1).toByte() }
    assertThrows(IllegalArgumentException::class.java) { edge.open(changed) }
    assertArrayEquals("hello".toByteArray(), edge.open(raw).plaintext)
  }
  @Test
  fun `chunks reassemble once and conflicting duplicate fails`() {
    val client = BlueCashBleFrameSession.client(secret, sessionId)
    val edge = BlueCashBleFrameSession.edge(secret, sessionId)
    val chunks = BlueCashBleFrameSession.chunks(ByteArray(400) { (it % 251).toByte() }, 185)
    val reassembler = BlueCashBleReassembler()
    var result: ByteArray? = null
    chunks.forEachIndexed { i, v ->
      val frame = edge.open(client.seal(id, i, chunks.size, 0, v))
      if (i < chunks.lastIndex) assertNull(reassembler.accept(frame))
      else result = reassembler.accept(frame)
    }
    assertArrayEquals(ByteArray(400) { (it % 251).toByte() }, result)
  }
}
