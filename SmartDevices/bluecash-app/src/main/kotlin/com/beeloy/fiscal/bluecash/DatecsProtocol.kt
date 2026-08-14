package com.beeloy.fiscal.bluecash

import java.io.InputStream
import java.io.OutputStream

data class DatecsResponse(
  val sequence: Int,
  val command: Int,
  val data: ByteArray,
  val status: ByteArray
)

object DatecsFrameCodec {
  private const val SOH = 1
  private const val EOT = 4
  private const val ENQ = 5
  private const val ETX = 3
  private fun putWord(out: MutableList<Byte>, value: Int) {
    for (shift in intArrayOf(12, 8, 4, 0)) out += (0x30 + ((value shr shift) and 15)).toByte()
  }
  private fun word(raw: ByteArray, at: Int): Int {
    var v = 0
    repeat(4) {
      val n = raw[at + it].toInt() - 0x30
      require(n in 0..15) { "DATECS_HEX" }
      v = (v shl 4) or n
    }
    return v
  }
  fun encode(sequence: Int, command: Int, data: ByteArray): ByteArray {
    require(sequence in 0x20..0x7f && command in 0..0xffff && data.size <= 496) {
      "DATECS_FRAME_FIELDS"
    }
    val out = mutableListOf(SOH.toByte())
    putWord(out, data.size + 0x26)
    out += sequence.toByte()
    putWord(out, command)
    out += data.toList()
    out += ENQ.toByte()
    val sum = out.drop(1).sumOf { it.toInt() and 255 } and 0xffff
    putWord(out, sum)
    out += ETX.toByte()
    return out.toByteArray()
  }
  fun decode(raw: ByteArray): DatecsResponse {
    require(raw.size >= 25 && raw[0].toInt() == SOH && raw.last().toInt() == ETX) {
      "DATECS_BAD_FRAME"
    }
    val dataSize = word(raw, 1) - 0x2f
    require(dataSize >= 0 && raw.size == 25 + dataSize) { "DATECS_LENGTH" }
    val separator = 10 + dataSize
    val checksumAt = 19 + dataSize
    require(raw[separator].toInt() == EOT && raw[checksumAt].toInt() == ENQ) { "DATECS_SEPARATOR" }
    val expected = (1 until checksumAt).sumOf { raw[it].toInt() and 255 } and 0xffff
    require(word(raw, checksumAt + 1) == expected) { "DATECS_BCC" }
    return DatecsResponse(
      raw[5].toInt() and 255,
      word(raw, 6),
      raw.copyOfRange(10, separator),
      raw.copyOfRange(separator + 1, checksumAt)
    )
  }
}

data class FiscalLine(
  val name: String,
  val taxGroup: Char,
  val unitPrice: String,
  val quantity: String,
  val discountType: Int = 0,
  val discountValue: String = "",
  val department: Int = 0,
  val unit: String = ""
)

data class DatecsStornoDocument(
  val reason: Int,
  val documentNumber: Long,
  val documentDateTime: String,
  val fiscalMemoryNumber: String,
  val originalUnp: String,
  val invoiceNumber: String? = null,
  val invoiceReason: String? = null
)

object DatecsPayloads {
  private val money = Regex("[0-9]{1,5}\\.[0-9]{2}")
  private val quantity = Regex("[0-9]{1,2}\\.[0-9]{3}")
  private fun bytes(vararg fields: String) =
    fields.joinToString("\t", postfix = "\t").toByteArray(Charsets.UTF_8)
  fun open(
    operator: Int,
    password: String,
    unp: String,
    till: Int,
    invoice: Boolean = false
  ): ByteArray {
    require(
      operator in 1..30 &&
        password.matches(Regex("[0-9]{1,8}")) &&
        unp.length in 8..64 &&
        till in 1..99999
    )
    return bytes("$operator", password, unp, "$till", if (invoice) "I" else "")
  }
  fun line(v: FiscalLine): ByteArray {
    require(
      v.name.isNotBlank() &&
        v.name.length <= 72 &&
        v.taxGroup in 'A'..'H' &&
        money.matches(v.unitPrice) &&
        quantity.matches(v.quantity) &&
        v.discountType in 0..4 &&
        v.department in 0..99
    )
    if (v.discountType != 0) require(money.matches(v.discountValue))
    return bytes(
      v.name,
      "${v.taxGroup-'A'+1}",
      v.unitPrice,
      v.quantity,
      if (v.discountType == 0) "" else "${v.discountType}",
      v.discountValue,
      "${v.department}",
      v.unit
    )
  }
  fun payment(mode: Int, amount: String): ByteArray {
    require(mode in 0..5 && money.matches(amount))
    return bytes("$mode", amount)
  }
  fun report(z: Boolean) = bytes(if (z) "Z" else "X")
  fun cash(out: Boolean, amount: String): ByteArray {
    require(money.matches(amount))
    return bytes(if (out) "1" else "0", amount)
  }
  /** Datecs command 43, Communication Protocol v2.11.4 section 4.6. */
  fun stornoOpen(
    operator: Int,
    password: String,
    till: Int,
    document: DatecsStornoDocument
  ): ByteArray {
    require(operator in 1..30 && password.matches(Regex("[0-9]{1,8}")) && till in 1..99999)
    require(document.reason in 0..2 && document.documentNumber in 1..9999999)
    require(
      document.documentDateTime.matches(
        Regex("[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}")
      )
    )
    require(document.fiscalMemoryNumber.matches(Regex("[0-9]{8}")))
    require(document.originalUnp.matches(Regex("[A-Z]{2}[0-9]{6}-[A-Za-z0-9]{4}-[0-9]{7}")))
    val invoice = document.invoiceNumber != null
    if (invoice)
      require(
        document.invoiceNumber!!.matches(Regex("[0-9]{1,10}")) &&
          !document.invoiceReason.isNullOrBlank() &&
          document.invoiceReason!!.length <= 64
      )
    else require(document.invoiceReason == null)
    return bytes(
      "$operator",
      password,
      "$till",
      "${document.reason}",
      "${document.documentNumber}",
      document.documentDateTime,
      document.fiscalMemoryNumber,
      if (invoice) "I" else "",
      document.invoiceNumber ?: "",
      document.invoiceReason ?: "",
      document.originalUnp
    )
  }
}

class DatecsSocketProtocol(private val input: InputStream, private val output: OutputStream) {
  private var sequence = 0x20
  @Synchronized
  fun execute(command: Int, payload: ByteArray = byteArrayOf()): DatecsResponse {
    val expected = sequence
    output.write(DatecsFrameCodec.encode(expected, command, payload))
    output.flush()
    val frame = ArrayList<Byte>()
    while (true) {
      val b = input.read()
      if (b < 0) error("DATECS_EOF")
      frame += b.toByte()
      if (b == 3) break
      require(frame.size <= 1024) { "DATECS_FRAME_TOO_LARGE" }
    }
    val result = DatecsFrameCodec.decode(frame.toByteArray())
    require(result.sequence == expected && result.command == command) { "DATECS_CORRELATION" }
    sequence = if (sequence == 0x7f) 0x20 else sequence + 1
    return result
  }
}
