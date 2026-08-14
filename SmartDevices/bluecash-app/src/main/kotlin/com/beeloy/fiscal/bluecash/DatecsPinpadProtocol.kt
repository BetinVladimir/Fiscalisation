package com.beeloy.fiscal.bluecash

import java.io.InputStream
import java.io.OutputStream
import java.time.LocalDate

data class BerTlv(val tag: Long, val value: ByteArray)

object DatecsBerTlv {
  fun encode(tags: List<BerTlv>): ByteArray {
    val out = mutableListOf<Byte>()
    for (t in tags) {
      val tagBytes =
        generateSequence(t.tag) { if (it > 255) it shr 8 else null }
          .toList()
          .map { (it and 255).toByte() }
          .reversed()
      out += tagBytes
      if (t.value.size < 128) out += t.value.size.toByte()
      else {
        require(t.value.size <= 65535)
        out += 0x82.toByte()
        out += (t.value.size shr 8).toByte()
        out += t.value.size.toByte()
      }
      out += t.value.toList()
    }
    return out.toByteArray()
  }
  fun decode(raw: ByteArray): List<BerTlv> {
    val out = mutableListOf<BerTlv>()
    var p = 0
    while (p < raw.size) {
      var tag = (raw[p++].toInt() and 255).toLong()
      if ((tag and 31) == 31L) {
        do {
          val b = raw[p++].toInt() and 255
          tag = (tag shl 8) or b.toLong()
        } while ((b and 128) != 0)
      }
      var len = raw[p++].toInt() and 255
      if ((len and 128) != 0) {
        val count = len and 127
        require(count in 1..2 && p + count <= raw.size)
        len = 0
        repeat(count) { len = (len shl 8) or (raw[p++].toInt() and 255) }
      }
      require(len <= 4096 && p + len <= raw.size)
      out += BerTlv(tag, raw.copyOfRange(p, p + len))
      p += len
    }
    return out
  }
  fun integer(value: ByteArray) =
    value.fold(0L) { a, b -> (a shl 8) or (b.toInt() and 255).toLong() }
}

class DatecsPinpadWire(private val input: InputStream, private val output: OutputStream) {
  data class Packet(val event: Int, val error: Int, val payload: ByteArray)
  fun command(command: Int, subcommand: Int, payload: ByteArray = byteArrayOf()): ByteArray {
    val body = byteArrayOf(subcommand.toByte()) + payload
    val frame =
      mutableListOf(
        0x3e.toByte(),
        command.toByte(),
        0,
        (body.size shr 8).toByte(),
        body.size.toByte()
      )
    frame += body.toList()
    frame += frame.fold(0) { a, b -> a xor (b.toInt() and 255) }.toByte()
    output.write(frame.toByteArray())
    output.flush()
    while (true) {
      val packet = readPacket()
      if (packet.event == 0) {
        require(packet.error == 0) { "DATECS_PINPAD_ERROR_${packet.error}" }
        return packet.payload
      }
    }
  }
  fun event(code: Int): ByteArray {
    while (true) {
      val packet = readPacket()
      if (packet.event == code) return packet.payload
    }
  }
  private fun readPacket(): Packet {
    while (true) {
      val first = input.read()
      if (first < 0) error("DATECS_PINPAD_EOF")
      if (first != 0x3e) continue
      val event = read()
      val error = read()
      val length = (read() shl 8) or read()
      require(length <= 4096)
      val payload = ByteArray(length) { read().toByte() }
      val bcc = read()
      var expected = 0x3e xor event xor error xor (length shr 8) xor (length and 255)
      payload.forEach { expected = expected xor (it.toInt() and 255) }
      require(bcc == expected) { "DATECS_PINPAD_BCC" }
      return Packet(event, error, payload)
    }
  }
  private fun read(): Int = input.read().also { if (it < 0) error("DATECS_PINPAD_EOF") }
}

data class OriginalCardTransaction(
  val operationId: String,
  val amountMinor: Long,
  val rrn: String,
  val authorizationCode: String,
  val state: String
)

interface CardTransactionStore {
  fun prepare(operationId: String, amountMinor: Long)
  fun approve(value: OriginalCardTransaction)
  fun original(operationId: String): OriginalCardTransaction?
  fun markReversed(operationId: String)
}

class MemoryCardTransactionStore : CardTransactionStore {
  private val rows = mutableMapOf<String, OriginalCardTransaction>()
  override fun prepare(operationId: String, amountMinor: Long) {
    val old = rows[operationId]
    require(old == null || (old.amountMinor == amountMinor && old.state == "PREPARED")) {
      "PAYMENT_ID_CONFLICT"
    }
    if (old == null)
      rows[operationId] = OriginalCardTransaction(operationId, amountMinor, "", "", "PREPARED")
  }
  override fun approve(value: OriginalCardTransaction) {
    rows[value.operationId] = value.copy(state = "APPROVED")
  }
  override fun original(operationId: String) = rows[operationId]
  override fun markReversed(operationId: String) {
    rows[operationId] = rows[operationId]!!.copy(state = "REVERSED")
  }
}

class BoricaPinpadCodec(
  private val today: () -> LocalDate = { LocalDate.now() },
  private val store: CardTransactionStore = MemoryCardTransactionStore()
) : DatecsPinpadCodec {
  override fun purchase(
    amountMinor: Long,
    operationId: String,
    input: InputStream,
    output: OutputStream
  ): Map<String, String> {
    require(amountMinor in 1..Int.MAX_VALUE)
    store.prepare(operationId, amountMinor)
    val wire = DatecsPinpadWire(input, output)
    runCatching { wire.command(64, 2, byteArrayOf(3, 0)) }
    wire.command(61, 3)
    val date = today()
    val bcd = byteArrayOf(bcd(date.year % 100), bcd(date.monthValue), bcd(date.dayOfMonth))
    val amount =
      byteArrayOf(
        (amountMinor shr 24).toByte(),
        (amountMinor shr 16).toByte(),
        (amountMinor shr 8).toByte(),
        amountMinor.toByte()
      )
    wire.command(
      61,
      1,
      byteArrayOf(1) + DatecsBerTlv.encode(listOf(BerTlv(0x9A, bcd), BerTlv(0x81, amount)))
    )
    val result = parseResult(wire.event(14))
    if (result["approved"] == "true")
      store.approve(
        OriginalCardTransaction(
          operationId,
          amountMinor,
          result["rrn"].orEmpty(),
          result["authorization_code"].orEmpty(),
          "APPROVED"
        )
      )
    return result
  }
  override fun reverse(
    operationId: String,
    input: InputStream,
    output: OutputStream
  ): Map<String, String> {
    val old =
      store.original(operationId)?.takeIf { it.state == "APPROVED" }
        ?: error("DATECS_ORIGINAL_PAYMENT_REQUIRED")
    val wire = DatecsPinpadWire(input, output)
    wire.command(61, 3)
    val tags =
      listOf(
        BerTlv(0x81, be4(old.amountMinor)),
        BerTlv(0xDF01, old.rrn.toByteArray(Charsets.US_ASCII)),
        BerTlv(0xDF02, old.authorizationCode.toByteArray(Charsets.US_ASCII))
      )
    wire.command(61, 1, byteArrayOf(7) + DatecsBerTlv.encode(tags))
    return parseResult(wire.event(14)).also {
      if (it["approved"] == "true") store.markReversed(operationId)
    }
  }
  private fun parseResult(event: ByteArray): Map<String, String> {
    require(event.isNotEmpty() && event[0].toInt() == 1) { "DATECS_PINPAD_EVENT" }
    val tags = DatecsBerTlv.decode(event.copyOfRange(1, event.size)).associateBy { it.tag }
    val result = tags[0xDF05]?.let { DatecsBerTlv.integer(it.value) } ?: 2
    val error = tags[0xDF06]?.let { DatecsBerTlv.integer(it.value) } ?: 0
    val host = tags[0xDF09]?.let { DatecsBerTlv.integer(it.value) } ?: 0
    return mapOf(
      "approved" to (result == 0L && error == 0L && host == 0L).toString(),
      "result" to "$result",
      "error_code" to if (host != 0L) "BORICA_$host" else if (error != 0L) "PINPAD_$error" else "",
      "rrn" to (tags[0xDF07]?.value?.toString(Charsets.US_ASCII) ?: ""),
      "authorization_code" to (tags[0xDF08]?.value?.toString(Charsets.US_ASCII) ?: "")
    )
  }
  private fun bcd(v: Int) = (((v / 10) shl 4) or (v % 10)).toByte()
  private fun be4(v: Long) =
    byteArrayOf((v shr 24).toByte(), (v shr 16).toByte(), (v shr 8).toByte(), v.toByte())
}
