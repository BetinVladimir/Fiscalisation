package com.beeloy.fiscal.bluecash

import java.io.ByteArrayOutputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.security.MessageDigest
import java.util.UUID

data class OpenMvpMessage(val id: UUID, val payload: ByteArray)

class BlueCashOpenMvpAssembler {
  private data class State(
    val total: Int,
    val digest: ByteArray,
    var next: Int = 0,
    val out: ByteArrayOutputStream = ByteArrayOutputStream()
  )
  private var state: Pair<UUID, State>? = null
  fun clear() {
    state = null
  }
  fun accept(frame: ByteArray): OpenMvpMessage? {
    require(frame.size > 57 && frame.copyOfRange(0, 4).contentEquals("BFF1".toByteArray())) {
      "BLE_FRAME_INVALID"
    }
    val b = ByteBuffer.wrap(frame).order(ByteOrder.BIG_ENDIAN).apply { position(4) }
    val id = UUID(b.long, b.long)
    val total = b.short.toInt() and 0xffff
    val offset = b.short.toInt() and 0xffff
    val final = b.get().toInt() == 1
    val digest = ByteArray(32).also { b.get(it) }
    val body = ByteArray(b.remaining()).also { b.get(it) }
    require(total in 1..8192 && offset + body.size <= total) { "BLE_FRAME_BOUNDS" }
    var current = state
    if (current == null) {
      require(offset == 0) { "BLE_FRAME_ORDER" }
      current = id to State(total, digest)
      state = current
    }
    val s = current.second
    require(
      current.first == id && s.total == total && s.next == offset && s.digest.contentEquals(digest)
    ) {
      clear()
      "BLE_FRAME_CONFLICT"
    }
    s.out.write(body)
    s.next += body.size
    if (!final) return null
    require(s.next == s.total) {
      clear()
      "BLE_FRAME_INCOMPLETE"
    }
    val payload = s.out.toByteArray()
    clear()
    require(MessageDigest.getInstance("SHA-256").digest(payload).contentEquals(digest)) {
      "BLE_FRAME_DIGEST"
    }
    return OpenMvpMessage(id, payload)
  }
}

object BlueCashOpenMvpFrames {
  fun encode(id: UUID, payload: ByteArray, mtu: Int): List<ByteArray> {
    require(payload.isNotEmpty() && payload.size <= 8192) { "BLE_INTENT_TOO_LARGE" }
    val capacity = mtu.coerceIn(64, 517) - 3 - 57
    require(capacity > 0) { "BLE_MTU_TOO_SMALL" }
    val digest = MessageDigest.getInstance("SHA-256").digest(payload)
    return payload.asList().chunked(capacity).mapIndexed { index, part ->
      val offset = index * capacity
      ByteBuffer.allocate(57 + part.size)
        .order(ByteOrder.BIG_ENDIAN)
        .put("BFF1".toByteArray())
        .putLong(id.mostSignificantBits)
        .putLong(id.leastSignificantBits)
        .putShort(payload.size.toShort())
        .putShort(offset.toShort())
        .put(if (offset + part.size == payload.size) 1 else 0)
        .put(digest)
        .put(part.toByteArray())
        .array()
    }
  }
}

class BlueCashOpenMvpChannel(
  private val execute: (Map<String, Any?>) -> Map<String, Any?>,
  private val mtu: () -> Int
) {
  private val assembler = BlueCashOpenMvpAssembler()
  fun close() = assembler.clear()
  fun command(frame: ByteArray): List<ByteArray> {
    val message = assembler.accept(frame) ?: return emptyList()
    val intent = BlueCashCanonicalCbor.decodeMap(message.payload)
    require(intent["intent_id"] == message.id.toString()) { "BLE_MESSAGE_ID_MISMATCH" }
    return BlueCashOpenMvpFrames.encode(
      message.id,
      BlueCashCanonicalCbor.encode(execute(intent)),
      mtu()
    )
  }
}
