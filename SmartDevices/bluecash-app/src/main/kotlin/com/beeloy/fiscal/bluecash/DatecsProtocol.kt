package com.beeloy.fiscal.bluecash

import java.io.IOException
import java.math.BigDecimal
import java.io.InputStream
import java.io.OutputStream
import java.nio.CharBuffer
import java.nio.charset.CharacterCodingException
import java.nio.charset.Charset
import java.nio.charset.CodingErrorAction
import java.time.Duration
import java.util.concurrent.Callable
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

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
    putWord(out, data.size + 0x2a)
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
    val dataSize = word(raw, 1) - 0x33
    require(dataSize in 0..480 && raw.size == 25 + dataSize) { "DATECS_LENGTH" }
    val separator = 10 + dataSize
    val checksumAt = 19 + dataSize
    require(raw[separator].toInt() == EOT && raw[checksumAt].toInt() == ENQ) { "DATECS_SEPARATOR" }
    val expected = (1..checksumAt).sumOf { raw[it].toInt() and 255 } and 0xffff
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

/**
 * VG-15: Response from Datecs cmd 70 (cash in/out, §4.30).
 * All amounts in the same decimal format as the fiscal device (e.g. "150.00").
 */
data class Cmd70Response(
    val cashIn: java.math.BigDecimal,
    val cashOut: java.math.BigDecimal,
    val cashBalance: java.math.BigDecimal
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

/** Encodes/decodes fiscal device text as CP1251 (invariant 7). */
object DatecsText {
  private val CHARSET: Charset = Charset.forName("windows-1251")
  fun encode(s: String): ByteArray {
    try {
      val buf = CHARSET.newEncoder()
        .onMalformedInput(CodingErrorAction.REPORT)
        .onUnmappableCharacter(CodingErrorAction.REPORT)
        .encode(CharBuffer.wrap(s))
      val ba = ByteArray(buf.limit())
      buf.get(ba)
      return ba
    } catch (e: CharacterCodingException) {
      throw IllegalArgumentException("DATECS_TEXT_UNMAPPABLE")
    }
  }
  fun decode(b: ByteArray): String = String(b, CHARSET)
}

object DatecsPayloads {
  private val money = Regex("[0-9]{1,5}\\.[0-9]{2}")
  private val quantity = Regex("[0-9]{1,5}\\.[0-9]{3}")
  // BG UNP format: 2 uppercase letters + 6 digits + hyphen + 4 alphanumeric + hyphen + 7 digits
  private val unpRegex = Regex("[A-Z]{2}[0-9]{6}-[A-Z0-9]{4}-[0-9]{7}")
  private val percentRegex = Regex("(100\\.00|[0-9]{1,2}\\.[0-9]{2})")
  private fun bytes(vararg fields: String) =
    DatecsText.encode(fields.joinToString("\t", postfix = "\t"))
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
        unpRegex.matches(unp) &&
        till in 1..99999
    ) { "OPEN_INVALID:operator=$operator,unp=$unp,till=$till" }
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
    if (v.discountType in 1..2) {
      // Percentage discount: must be 0.01-100.00
      require(percentRegex.matches(v.discountValue)) { "PAYLOAD_INVALID:discount_percent" }
      val pct = v.discountValue.toBigDecimal()
      require(pct >= java.math.BigDecimal("0.01") && pct <= java.math.BigDecimal("100.00")) {
        "PAYLOAD_INVALID:discount_percent_range"
      }
    } else if (v.discountType in 3..4) {
      require(money.matches(v.discountValue)) { "PAYLOAD_INVALID:discount_value" }
    }
    // VG-13: always use syntax №2 (8 fields) with Unit="бр." as default per §4.12.
    // CP1251 bytes for "бр.": 0xE1 0xF0 0x2E
    val unit = v.unit.ifEmpty { "бр." }
    return bytes(
      v.name,
      "${v.taxGroup-'A'+1}",
      v.unitPrice,
      v.quantity,
      if (v.discountType == 0) "" else "${v.discountType}",
      v.discountValue,
      "${v.department}",
      unit
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
  /** Datecs Communication Protocol v2.11.4 §4.21 — cancel an open fiscal receipt. */
  const val CMD_CANCEL_RECEIPT = 60
  fun cancelReceipt(): ByteArray = byteArrayOf()

  /**
   * VG-15: Parse a cmd 70 (cash in/out, §4.30) response.
   *
   * Response format (tab-separated, CP1251):
   *   [0] error code (0 = OK)
   *   [1] cash-in total (decimal string, e.g. "150.00")
   *   [2] cash-out total (decimal string)
   *   [3] current cash balance (decimal string)
   *
   * @return [Cmd70Response] or null if the response cannot be parsed.
   */
  fun parseCashInOutResponse(response: DatecsResponse): Cmd70Response? {
    val text = DatecsText.decode(response.data)
    val fields = text.split('\t')
    val errorCode = fields.getOrNull(0)?.trim()?.toIntOrNull() ?: return null
    if (errorCode != 0) return null
    val cashIn      = fields.getOrNull(1)?.trim()?.toBigDecimalOrNull() ?: return null
    val cashOut     = fields.getOrNull(2)?.trim()?.toBigDecimalOrNull() ?: return null
    val cashBalance = fields.getOrNull(3)?.trim()?.toBigDecimalOrNull() ?: return null
    return Cmd70Response(cashIn, cashOut, cashBalance)
  }

  /**
   * VG-15: Validate that a cash-out amount does not exceed the available cash balance.
   *
   * @param amountStr  decimal string (money regex), e.g. "13.67"
   * @param balance    current cash balance from [Cmd70Response.cashBalance]
   * @throws IllegalArgumentException("CASH_OUT_EXCEEDS_BALANCE") if amount > balance
   */
  fun requireCashAvailable(amountStr: String, balance: BigDecimal) {
    require(money.matches(amountStr)) { "PAYLOAD_INVALID:cash_amount" }
    val amount = BigDecimal(amountStr)
    require(amount <= balance) { "CASH_OUT_EXCEEDS_BALANCE" }
  }

  private val MAX_LINE_TOTAL = BigDecimal("9999999.99")

  /** Returns PAYLOAD_INVALID:<field> codes for every invalid line/payment; empty = OK. */
  fun validateSale(lines: List<FiscalLine>, payments: List<SalePayment>): List<String> {
    val errors = mutableListOf<String>()
    for (line in lines) {
      if (line.name.isBlank() || line.name.length > 72) errors += "PAYLOAD_INVALID:name"
      if (!money.matches(line.unitPrice)) errors += "PAYLOAD_INVALID:unit_price"
      if (!quantity.matches(line.quantity)) errors += "PAYLOAD_INVALID:quantity"
      if (line.taxGroup !in 'A'..'H') errors += "PAYLOAD_INVALID:tax_group"
      if (line.discountType !in 0..4) errors += "PAYLOAD_INVALID:discount_type"
      if (errors.isEmpty()) {
        val lineTotal = BigDecimal(line.unitPrice) * BigDecimal(line.quantity)
        if (lineTotal > MAX_LINE_TOTAL) errors += "PAYLOAD_INVALID:line_total_exceeded"
      }
    }
    for (p in payments) {
      if (!money.matches(p.amount)) errors += "PAYLOAD_INVALID:payment_amount"
    }
    // VG-06.3: validate plan totals match before any I/O (only when no line discounts)
    if (errors.isEmpty() && lines.isNotEmpty() && payments.isNotEmpty() &&
        lines.all { it.discountType == 0 }) {
      val lineSum = lines.sumOf { BigDecimal(it.unitPrice) * BigDecimal(it.quantity) }
      val paySum = payments.sumOf { BigDecimal(it.amount) }
      if (lineSum.compareTo(paySum) != 0) errors += "PAYLOAD_INVALID:total_mismatch"
    }
    return errors
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

/**
 * TG-006: Typed response data classes for Datecs commands.
 * Invariant 10: parsers never throw externally — they return null on format/logic errors.
 * Invariant 6: fiscal references come only from FU response, never synthesised.
 */

/** Cmd 48: Open fiscal receipt — error code only, no document number in response. */
data class Cmd48Response(val errorCode: Int)

/** Cmd 49: Add fiscal line — error code only. */
data class Cmd49Response(val errorCode: Int)

/** Cmd 53: Payment in fiscal receipt — error code and optional fields. */
data class Cmd53Response(val errorCode: Int)

/**
 * Cmd 56: Close fiscal receipt.
 * Response: field[0]=errorCode, field[1]=fiscal document reference (non-blank on success).
 */
data class Cmd56Response(val fiscalRef: String)

/**
 * Cmd 43: Open storno fiscal receipt — error code only.
 * BC-50: cmd43 is used for storno (sторно cmd43, внешний терминал).
 */
data class Cmd43Response(val errorCode: Int)

/**
 * Cmd 69/70 daily report response.
 * Response: field[0]=errorCode, field[1]=reportCount, field[2..5]=totals A/B/C/D.
 */
data class Cmd69Response(
    val reportCount: String,
    val totalA: String,
    val totalB: String,
    val totalC: String,
    val totalD: String
)

/**
 * Cmd 38: Print fiscal sub-total (intermediate total without closing receipt).
 * Response: field[0]=errorCode.
 */
data class Cmd38Response(val errorCode: Int)

/**
 * Cmd 39: Print receipt total / close with sub-total.
 * Response: field[0]=errorCode.
 */
data class Cmd39Response(val errorCode: Int)

/**
 * Cmd 42: Print department sub-total.
 * Response: field[0]=errorCode.
 */
data class Cmd42Response(val errorCode: Int)

// ── Invariant-10-safe parsers (return null on any format failure) ──────────

private fun DatecsResponse.firstField(): Int? =
    DatecsText.decode(data).split('\t').getOrNull(0)?.trim()?.toIntOrNull()

/** Returns null when data is empty or errorCode is not an integer. */
fun DatecsResponse.parseCmd48(): Cmd48Response? =
    firstField()?.let { Cmd48Response(it) }

fun DatecsResponse.parseCmd49(): Cmd49Response? =
    firstField()?.let { Cmd49Response(it) }

fun DatecsResponse.parseCmd53(): Cmd53Response? =
    firstField()?.let { Cmd53Response(it) }

/**
 * Cmd 56: Returns Cmd56Response only when errorCode==0 AND field[1] is non-blank.
 * Invariant 6: no fiscal ref → null (caller must treat as RECOVERY_REQUIRED).
 */
fun DatecsResponse.parseCmd56(): Cmd56Response? {
    val fields = DatecsText.decode(data).split('\t')
    val err = fields.getOrNull(0)?.trim()?.toIntOrNull() ?: return null
    if (err != 0) return null
    val ref = fields.getOrNull(1)?.trim()?.takeIf { it.isNotBlank() } ?: return null
    return Cmd56Response(ref)
}

fun DatecsResponse.parseCmd43(): Cmd43Response? =
    firstField()?.let { Cmd43Response(it) }

/**
 * Cmd 69/70 report: returns Cmd69Response only when errorCode==0 and field[1] present.
 */
fun DatecsResponse.parseCmd69(): Cmd69Response? {
    val fields = DatecsText.decode(data).split('\t')
    val err = fields.getOrNull(0)?.trim()?.toIntOrNull() ?: return null
    if (err != 0) return null
    val n = fields.getOrNull(1)?.trim()?.takeIf { it.isNotBlank() } ?: return null
    return Cmd69Response(
        reportCount = n,
        totalA = fields.getOrNull(2)?.trim() ?: "",
        totalB = fields.getOrNull(3)?.trim() ?: "",
        totalC = fields.getOrNull(4)?.trim() ?: "",
        totalD = fields.getOrNull(5)?.trim() ?: ""
    )
}

fun DatecsResponse.parseCmd38(): Cmd38Response? =
    firstField()?.let { Cmd38Response(it) }

fun DatecsResponse.parseCmd39(): Cmd39Response? =
    firstField()?.let { Cmd39Response(it) }

fun DatecsResponse.parseCmd42(): Cmd42Response? =
    firstField()?.let { Cmd42Response(it) }

object DatecsTimeouts {
  private val REPORT = setOf(69, 70)
  private val RECEIPT = setOf(38, 39, 42, 43, 48, 49, 53, 56, 57, 60)
  fun forCommand(cmd: Int): Duration = when (cmd) {
    in REPORT -> Duration.ofSeconds(120)
    in RECEIPT -> Duration.ofSeconds(30)
    else -> Duration.ofSeconds(5)
  }
}

/**
 * Validates a DatecsResponse: requires at least [expectedFields] tab-separated fields in the data,
 * and that the first field is the decimal string "0" (no error). (VG-02.3)
 *
 * Empty data always fails when expectedFields > 0.
 * Non-zero first field throws DatecsFailure.Rejected with its integer value.
 */
fun DatecsResponse.requireOk(expectedFields: Int = 1): DatecsResponse {
    require(expectedFields >= 1) { "expectedFields must be >= 1" }
    val text = DatecsText.decode(data)
    if (text.isEmpty()) throw DatecsFailure.Rejected(0, "DATECS_EMPTY_RESPONSE")
    val fields = text.split('\t')
    if (fields.size < expectedFields) {
        throw DatecsFailure.Rejected(0, "DATECS_MISSING_FIELDS:expected=$expectedFields,got=${fields.size}")
    }
    val errorCode = fields[0].toIntOrNull()
        ?: throw DatecsFailure.Rejected(0, "DATECS_ERROR_CODE_NOT_INT:${fields[0]}")
    if (errorCode != 0) throw DatecsFailure.Rejected(errorCode, "DATECS_ERROR:$errorCode")
    return this
}

/**
 * Parsed representation of the 8 status bytes returned by the fiscal device (VG-02.1).
 *
 * Datecs status bytes layout (FP-800 / FP-2000 series):
 *   Byte 0 (S0): bit5=error, bit4=fiscal-memory-full, bit1=near-paper-end, bit0=paper-out
 *   Byte 2 (S2): bit5=error, bit4=open-fiscal-memory, bit1=payment-open
 *   Bytes 1,3,5,7 are reserved/device-specific
 * fatal = any error bit (bit5) set in any status byte
 * warning = any non-fatal advisory bit set (e.g. near-paper-end)
 */
data class DatecsStatus(val raw: ByteArray) {
    init { require(raw.size == 8) { "DatecsStatus: expected 8 bytes, got ${raw.size}" } }

    /** True if any byte has the error flag (bit 5 = 0x20) set. */
    val fatal: Boolean get() = raw.any { (it.toInt() and 0x20) != 0 }

    /** True if advisory flags are set (near-paper-end bit1 of byte0, or fiscal-memory-full bit4). */
    val warning: Boolean get() =
        (raw[0].toInt() and 0x02) != 0 ||   // near end of paper
        (raw[0].toInt() and 0x10) != 0       // fiscal memory almost full

    companion object {
        fun parse(bytes: ByteArray): DatecsStatus = DatecsStatus(bytes)
    }

    override fun equals(other: Any?) = other is DatecsStatus && raw.contentEquals(other.raw)
    override fun hashCode() = raw.contentHashCode()
}

/**
 * Types defined here (BUG-028) so callers like BlueCashEngine can use
 * `when` pattern matching instead of error-message text.
 */
sealed class DatecsFailure(msg: String) : RuntimeException(msg) {
  /** Command was sent (after flush) but communication failed — we don't know if FU processed it. */
  class Unknown(msg: String) : DatecsFailure(msg)
  /** Exception thrown before flush — command was NOT sent to the device. */
  class NotSent(val phase: String, msg: String) : DatecsFailure(msg)
  /** Response parsed, but fiscal device returned a non-zero error code. */
  class Rejected(val keyCode: Int, msg: String) : DatecsFailure(msg)
  /** Frame-level protocol errors — response received but couldn't be decoded. */
  class BadFrame(msg: String) : DatecsFailure(msg)
  class BccError(msg: String) : DatecsFailure(msg)
  class CorrelationError(msg: String) : DatecsFailure(msg)
  class FrameTooLarge(msg: String) : DatecsFailure(msg)
}

/** Persists the last Datecs SEQ counter across restarts (VG-03.2). */
interface SequenceStore {
  fun load(): Int?   // null = unknown (first boot or cleared)
  fun save(seq: Int)
}

/** In-memory SequenceStore for tests. */
class MemorySequenceStore : SequenceStore {
  private var value: Int? = null
  override fun load(): Int? = value
  override fun save(seq: Int) { value = seq }
}

class DatecsSocketProtocol(
  private val input: InputStream,
  private val output: OutputStream,
  private val sequenceStore: SequenceStore? = null,
  internal val timeoutFor: (Int) -> Long = { cmd -> DatecsTimeouts.forCommand(cmd).toMillis() }
) {
  /** True when SEQ was unknown at construction — first execute() should be cmd 74 (read-only). */
  internal var seqUnknown: Boolean = sequenceStore?.load() == null
  internal var sequence = sequenceStore?.load() ?: 0x20

  private companion object {
    val READER = Executors.newCachedThreadPool { r ->
      Thread(r).also { it.isDaemon = true; it.name = "datecs-reader" }
    }
  }

  private data class ResponseOrNak(val isNak: Boolean, val frame: ByteArray?)

  private fun readResponse(): ResponseOrNak {
    val frame = ArrayList<Byte>()
    while (true) {
      val b = input.read()
      if (b < 0) throw DatecsFailure.Unknown("DATECS_EOF")
      when {
        b == 0x15 && frame.isEmpty() -> return ResponseOrNak(isNak = true, frame = null)
        b == 0x16 -> frame.clear()
        frame.isEmpty() && b != 1 -> Unit  // garbage before SOH
        else -> {
          frame += b.toByte()
          if (b == 3) break
          if (frame.size > 1024) throw DatecsFailure.FrameTooLarge("DATECS_FRAME_TOO_LARGE")
        }
      }
    }
    return ResponseOrNak(isNak = false, frame = frame.toByteArray())
  }

  @Synchronized
  fun execute(command: Int, payload: ByteArray = byteArrayOf()): DatecsResponse {
    val expected = sequence
    val encodedFrame = try {
      DatecsFrameCodec.encode(expected, command, payload)
    } catch (e: IllegalArgumentException) {
      throw DatecsFailure.NotSent("encode", e.message ?: "DATECS_FRAME_FIELDS")
    }
    try {
      output.write(encodedFrame)
      output.flush()
    } catch (e: IOException) {
      throw DatecsFailure.NotSent("write", e.message ?: "DATECS_WRITE_ERROR")
    } catch (e: SecurityException) {
      throw DatecsFailure.NotSent("write", "DATECS_WRITE_DENIED")
    }
    val timeoutMs = timeoutFor(command)
    var retransmits = 0
    while (true) {
      val future = READER.submit(Callable { readResponse() })
      val response: ResponseOrNak
      try {
        response = future.get(timeoutMs, TimeUnit.MILLISECONDS)
      } catch (e: TimeoutException) {
        future.cancel(true)
        runCatching { input.close() }
        throw DatecsFailure.Unknown("DATECS_TIMEOUT")
      } catch (e: ExecutionException) {
        val cause = e.cause
        if (cause is DatecsFailure) throw cause
        throw DatecsFailure.Unknown(cause?.message ?: "DATECS_UNKNOWN")
      }
      if (response.isNak) {
        if (++retransmits >= 3) throw DatecsFailure.Unknown("DATECS_NAK_MAX_RETRANSMIT")
        try {
          output.write(encodedFrame)
          output.flush()
        } catch (e: IOException) {
          throw DatecsFailure.Unknown("DATECS_RETRANSMIT_FAILED")
        }
        continue
      }
      val result = try {
        DatecsFrameCodec.decode(response.frame!!)
      } catch (e: IllegalArgumentException) {
        throw DatecsFailure.BadFrame(e.message ?: "DATECS_DECODE_ERROR")
      }
      if (result.sequence != expected || result.command != command) {
        throw DatecsFailure.CorrelationError("DATECS_CORRELATION")
      }
      sequence = if (sequence == 0x7f) 0x20 else sequence + 1
      sequenceStore?.save(sequence)
      seqUnknown = false
      return result
    }
  }
}
