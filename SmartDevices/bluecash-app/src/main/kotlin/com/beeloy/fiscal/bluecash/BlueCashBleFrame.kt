package com.beeloy.fiscal.bluecash

import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.security.MessageDigest
import java.util.UUID
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

data class BlueCashBleFrame(
  val messageId: UUID,
  val counter: Long,
  val chunkIndex: Int,
  val chunkCount: Int,
  val flags: Int,
  val plaintext: ByteArray
)

/** BeeFiscal BLE v1 binary framing, directional key derivation and replay protection. */
class BlueCashBleFrameSession
private constructor(
  private val txKey: ByteArray,
  private val rxKey: ByteArray,
  private val txPrefix: ByteArray,
  private val rxPrefix: ByteArray
) {
  companion object {
    const val HEADER_SIZE = 34
    const val TAG_SIZE = 16
    fun edge(secret: ByteArray, sessionId: String) = endpoint(secret, sessionId, false)
    fun client(secret: ByteArray, sessionId: String) = endpoint(secret, sessionId, true)
    private fun endpoint(
      secret: ByteArray,
      sessionId: String,
      client: Boolean
    ): BlueCashBleFrameSession {
      require(secret.size == 32 && sessionId.isNotBlank()) { "BLE_SESSION_KEY_INVALID" }
      val c2e = hmac(secret, "$sessionId:client-to-edge".toByteArray())
      val e2c = hmac(secret, "$sessionId:edge-to-client".toByteArray())
      val c2ePrefix = sha("c2e:$sessionId".toByteArray()).copyOf(4)
      val e2cPrefix = sha("e2c:$sessionId".toByteArray()).copyOf(4)
      return if (client) BlueCashBleFrameSession(c2e, e2c, c2ePrefix, e2cPrefix)
      else BlueCashBleFrameSession(e2c, c2e, e2cPrefix, c2ePrefix)
    }
    private fun hmac(key: ByteArray, value: ByteArray) =
      Mac.getInstance("HmacSHA256").run {
        init(SecretKeySpec(key, "HmacSHA256"))
        doFinal(value)
      }
    private fun sha(value: ByteArray) = MessageDigest.getInstance("SHA-256").digest(value)
    fun maxChunkPlaintext(attMtu: Int) =
      (attMtu - 3 - HEADER_SIZE - TAG_SIZE).also { require(it >= 1) { "BLE_MTU_TOO_SMALL" } }
    fun chunks(payload: ByteArray, attMtu: Int): List<ByteArray> {
      val max = maxChunkPlaintext(attMtu)
      if (payload.isEmpty()) return listOf(byteArrayOf())
      val result = payload.asList().chunked(max).map { it.toByteArray() }
      require(result.size <= 65535) { "BLE_MESSAGE_TOO_LARGE" }
      return result
    }
  }
  private var txCounter = 0L
  private var rxCounter = 0L
  @Synchronized
  fun seal(
    messageId: UUID,
    chunkIndex: Int,
    chunkCount: Int,
    flags: Int,
    plain: ByteArray
  ): ByteArray {
    require(
      chunkCount in 1..65535 &&
        chunkIndex in 0 until chunkCount &&
        flags in 0..255 &&
        plain.size <= 65535
    ) {
      "BLE_CHUNK_INVALID"
    }
    require(txCounter < Long.MAX_VALUE) { "BLE_COUNTER_EXHAUSTED" }
    txCounter++
    val header =
      ByteBuffer.allocate(HEADER_SIZE)
        .order(ByteOrder.BIG_ENDIAN)
        .put('B'.code.toByte())
        .put('F'.code.toByte())
        .put(1)
        .put(flags.toByte())
        .putLong(messageId.mostSignificantBits)
        .putLong(messageId.leastSignificantBits)
        .putLong(txCounter)
        .putShort(chunkIndex.toShort())
        .putShort(chunkCount.toShort())
        .putShort(plain.size.toShort())
        .array()
    val sealed = crypt(Cipher.ENCRYPT_MODE, txKey, nonce(txPrefix, txCounter), header, plain)
    return header + sealed
  }
  @Synchronized
  fun open(raw: ByteArray): BlueCashBleFrame {
    require(
      raw.size >= HEADER_SIZE + TAG_SIZE &&
        raw[0] == 'B'.code.toByte() &&
        raw[1] == 'F'.code.toByte() &&
        raw[2].toInt() == 1
    ) {
      "BLE_FRAME_INVALID"
    }
    val header = raw.copyOfRange(0, HEADER_SIZE)
    val b = ByteBuffer.wrap(header).order(ByteOrder.BIG_ENDIAN)
    b.position(3)
    val flags = b.get().toInt() and 255
    val id = UUID(b.long, b.long)
    val counter = b.long
    val index = b.short.toInt() and 65535
    val count = b.short.toInt() and 65535
    val length = b.short.toInt() and 65535
    require(counter > rxCounter) { "BLE_FRAME_REPLAY" }
    require(count >= 1 && index < count && raw.size == HEADER_SIZE + length + TAG_SIZE) {
      "BLE_FRAME_INVALID"
    }
    val plain =
      runCatching {
          crypt(
            Cipher.DECRYPT_MODE,
            rxKey,
            nonce(rxPrefix, counter),
            header,
            raw.copyOfRange(HEADER_SIZE, raw.size)
          )
        }
        .getOrElse { throw IllegalArgumentException("BLE_FRAME_BAD_TAG") }
    rxCounter = counter
    return BlueCashBleFrame(id, counter, index, count, flags, plain)
  }
  private fun nonce(prefix: ByteArray, counter: Long) =
    ByteBuffer.allocate(12).order(ByteOrder.BIG_ENDIAN).put(prefix).putLong(counter).array()
  private fun crypt(mode: Int, key: ByteArray, nonce: ByteArray, aad: ByteArray, value: ByteArray) =
    Cipher.getInstance("AES/GCM/NoPadding").run {
      init(mode, SecretKeySpec(key, "AES"), GCMParameterSpec(128, nonce))
      updateAAD(aad)
      doFinal(value)
    }
}

class BlueCashBleReassembler(
  private val maxMessages: Int = 8,
  private val maxBytes: Int = 256 * 1024
) {
  private data class Pending(
    val count: Int,
    val parts: MutableMap<Int, ByteArray> = mutableMapOf(),
    var bytes: Int = 0
  )
  private val pending = linkedMapOf<UUID, Pending>()
  @Synchronized
  fun accept(frame: BlueCashBleFrame): ByteArray? {
    require(pending.containsKey(frame.messageId) || pending.size < maxMessages) {
      "BLE_FLOW_WINDOW_EXCEEDED"
    }
    val message = pending.getOrPut(frame.messageId) { Pending(frame.chunkCount) }
    require(message.count == frame.chunkCount) { "BLE_CHUNK_CONFLICT" }
    val prior = message.parts[frame.chunkIndex]
    require(prior == null || prior.contentEquals(frame.plaintext)) { "BLE_CHUNK_CONFLICT" }
    if (prior == null) {
      message.parts[frame.chunkIndex] = frame.plaintext.copyOf()
      message.bytes += frame.plaintext.size
    }
    require(message.bytes <= maxBytes) { "BLE_MESSAGE_TOO_LARGE" }
    if (message.parts.size < message.count) return null
    val result =
      (0 until message.count).fold(byteArrayOf()) { all, index ->
        all + (message.parts[index] ?: error("BLE_CHUNK_MISSING"))
      }
    pending.remove(frame.messageId)
    return result
  }
  @Synchronized
  fun clear() {
    pending.clear()
  }
}
