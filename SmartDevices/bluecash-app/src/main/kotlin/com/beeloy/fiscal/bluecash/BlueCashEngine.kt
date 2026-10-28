package com.beeloy.fiscal.bluecash

import java.security.MessageDigest
import java.time.Instant
import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock

data class SalePayment(val id: String, val type: String, val amount: String)

data class BlueCashSale(
  val operationId: String,
  val unp: String,
  val operator: Int,
  val password: String,
  val till: Int,
  val lines: List<FiscalLine>,
  val payments: List<SalePayment>
)

data class BlueCashReversal(
  val operationId: String,
  val originalOperationId: String,
  val operator: Int,
  val password: String,
  val till: Int,
  val originalDocument: DatecsStornoDocument,
  val lines: List<FiscalLine>,
  val payments: List<SalePayment>
)

data class JournalRecord(
  val sequence: Long,
  val operationId: String,
  val type: String,
  val occurredAt: String,
  val payload: String,
  val previousHash: String?,
  val hash: String,
  val signature: String,
  val acknowledged: Boolean = false
)

interface TransactionSigner {
  val keyId: String
  fun sign(hash: ByteArray): ByteArray
}

/**
 * Durable boundary shared by MQTT, local HTTP and BLE. Implementations reserve an operation before
 * physical I/O and retain unacknowledged records regardless of age.
 */
interface TransactionJournal {
  fun find(operationId: String): List<JournalRecord>
  fun append(operationId: String, type: String, payload: String): JournalRecord
  fun pending(limit: Int = 100): List<JournalRecord>
  fun acknowledge(throughSequence: Long, hash: String, ackId: String)
  fun purgeBefore(cutoff: Instant): Int
  fun checkpoint(): Pair<Long, String?>
}

interface DatecsFiscalPort {
  fun execute(command: Int, payload: ByteArray = byteArrayOf()): DatecsResponse
  fun reachable(): Boolean
}

interface DatecsPaymentPort {
  fun purchaseEur(amountMinor: Long, operationId: String): Map<String, String>
  fun reverse(operationId: String): Map<String, String>
  fun reachable(): Boolean
}

/** In-memory reference journal used by deterministic unit tests; production uses SQLite. */
class SignedMemoryJournal(
  private val signer: TransactionSigner,
  private val clock: () -> Instant = { Instant.now() }
) : TransactionJournal {
  private val rows = mutableListOf<JournalRecord>()
  private var acknowledgedThrough = 0L
  override fun find(operationId: String) = rows.filter { it.operationId == operationId }
  override fun append(operationId: String, type: String, payload: String): JournalRecord {
    synchronized(rows) {
      val seq = (rows.lastOrNull()?.sequence ?: 0) + 1
      val previous = rows.lastOrNull()?.hash
      val at = clock().toString()
      val canonical = "$seq\n$operationId\n$type\n$at\n${previous?:""}\n$payload"
      val hash =
        MessageDigest.getInstance("SHA-256").digest(canonical.toByteArray()).joinToString("") {
          "%02x".format(it)
        }
      val sig =
        java.util.Base64.getUrlEncoder()
          .withoutPadding()
          .encodeToString(signer.sign(hash.chunked(2).map { it.toInt(16).toByte() }.toByteArray()))
      return JournalRecord(
          seq,
          operationId,
          type,
          at,
          payload,
          previous,
          hash,
          "${signer.keyId}:$sig"
        )
        .also { rows += it }
    }
  }
  override fun pending(limit: Int) = rows.filter { it.sequence > acknowledgedThrough }.take(limit)
  private var acknowledgedHash: String? = null
  override fun acknowledge(throughSequence: Long, hash: String, ackId: String) {
    synchronized(rows) {
      require(throughSequence >= acknowledgedThrough && ackId.isNotBlank()) { "ACK_INVALID" }
      val stored = rows.firstOrNull { it.sequence == throughSequence }
      require(stored != null) { "ACK_SEQUENCE_UNKNOWN" }
      require(stored.hash == hash) { "ACK_HASH_MISMATCH" }
      acknowledgedThrough = throughSequence
      acknowledgedHash = hash
    }
  }
  override fun purgeBefore(cutoff: Instant): Int {
    synchronized(rows) {
      val removable =
        rows
          .takeWhile {
            it.sequence < acknowledgedThrough && Instant.parse(it.occurredAt).isBefore(cutoff)
          }
          .size
      if (removable > 0) rows.subList(0, removable).clear()
      return removable
    }
  }
  override fun checkpoint() = acknowledgedThrough to acknowledgedHash
}

/**
 * Executes fiscal and card commands as one recoverable saga. Replays return the journaled terminal
 * result; an interrupted EXECUTING record becomes UNKNOWN and must be reconciled by lookup.
 */
class BlueCashCommandProcessor(
  private val fiscal: DatecsFiscalPort,
  private val card: DatecsPaymentPort,
  private val journal: TransactionJournal
) {
  /** Single lock for sale/reverse/printerTest — one FU, one executor (BUG-072). */
  private val lock = ReentrantLock()

  companion object {
    /** Terminal states for sale idempotency check (BUG-025). */
    val SALE_TERMINAL = setOf(
      "FISCALIZED", "FAILED", "UNKNOWN", "COMPENSATED", "RECOVERY_REQUIRED"
    )
  }
  fun fiscalReachable() = fiscal.reachable()
  fun paymentReachable() = card.reachable()

  /**
   * TG-005: Cancel any dangling open receipt at startup before accepting new commands.
   *
   * Reads FU status (cmd 76, field[1] = receipt-open flag). If open, sends CMD_CANCEL_RECEIPT
   * (cmd 60) and records the event in the journal. Must be called once after connecting to the FU.
   *
   * @return true if a dangling receipt was found and cancel was attempted.
   */
  fun startup(recoveryOperationId: String): Boolean {
    require(recoveryOperationId.isNotBlank())
    val r76 = fiscal.execute(76)
    val text = String(r76.data, Charsets.ISO_8859_1)
    val isOpen = text.split('\t').getOrElse(1) { "0" }.toIntOrNull() ?: 0
    if (isOpen == 0) return false
    return try {
      fiscal.execute(DatecsPayloads.CMD_CANCEL_RECEIPT, DatecsPayloads.cancelReceipt())
      journal.append(recoveryOperationId, "STARTUP_RECEIPT_CANCELED", "isOpen=$isOpen")
      true
    } catch (e: DatecsFailure) {
      journal.append(recoveryOperationId, "STARTUP_RECEIPT_CANCEL_FAILED", "error=${e.message}")
      false
    }
  }

  fun administrative(
    operationId: String,
    command: Int,
    payload: ByteArray,
    acceptedEvidence: String = "state=ACCEPTED"
  ): Map<String, String> {
    require(operationId.isNotBlank() && command in setOf(69, 70))
    val old = journal.find(operationId)
    old
      .firstOrNull { it.type == "ACCEPTED" }
      ?.let { require(it.payload == acceptedEvidence) { "COMMAND_ID_PAYLOAD_CONFLICT" } }
    old
      .lastOrNull { it.type in setOf("FISCALIZED", "FAILED", "UNKNOWN") }
      ?.let {
        return parse(it.payload)
      }
    if (old.any { it.type == "ACCEPTED" || it.type == "EXECUTING" })
      return mapOf("state" to "UNKNOWN", "error_code" to "RECOVERY_REQUIRED")
    require(fiscal.reachable()) { "FISCAL_DEVICE_UNREACHABLE" }
    journal.append(operationId, "ACCEPTED", acceptedEvidence)
    journal.append(operationId, "EXECUTING", "state=EXECUTING")
    return try {
      ok(fiscal.execute(command, payload))
      mapOf("state" to "FISCALIZED").also { journal.append(operationId, "FISCALIZED", encode(it)) }
    } catch (e: Throwable) {
      val unknown = classified(e) == Outcome.UNKNOWN
      val result =
        mapOf(
          "state" to if (unknown) "UNKNOWN" else "FAILED",
          "error_code" to (e.message ?: "DATECS_FAILURE")
        )
      journal.append(operationId, result.getValue("state"), encode(result))
      result
    }
  }
  /**
   * Execute X (preview) or Z (close-of-day) report with durable journal.
   *
   * Idempotency: ERR_Z_REPORT_CLOSED (-104006) means Z was already executed on the FU;
   * we treat that as "already FISCALIZED" so the saga is idempotent.
   * Response fields (tab-separated): nRep, TotA, TotB, TotC, TotD, TotE, TotF, TotG, TotH, ...
   *
   * @param zReport  true = Z (closes day), false = X (read-only totals).
   */
  fun dailyReport(
    operationId: String,
    zReport: Boolean
  ): Map<String, String> {
    require(operationId.isNotBlank())
    val reportType = if (zReport) "Z" else "X"
    val acceptedEvidence = "state=ACCEPTED,report=$reportType"
    val old = journal.find(operationId)
    old.firstOrNull { it.type == "ACCEPTED" }
      ?.let { require(it.payload == acceptedEvidence) { "COMMAND_ID_PAYLOAD_CONFLICT" } }
    old.lastOrNull { it.type in setOf("FISCALIZED", "FAILED", "UNKNOWN") }
      ?.let { return parse(it.payload) }
    if (old.any { it.type == "ACCEPTED" || it.type == "EXECUTING" })
      return mapOf("state" to "UNKNOWN", "error_code" to "RECOVERY_REQUIRED")
    require(fiscal.reachable()) { "FISCAL_DEVICE_UNREACHABLE" }
    journal.append(operationId, "ACCEPTED", acceptedEvidence)
    journal.append(operationId, "EXECUTING", "state=EXECUTING")
    return try {
      val resp = fiscal.execute(69, DatecsPayloads.report(zReport))
      val text = String(resp.data, Charsets.ISO_8859_1)
      val fields = text.trimEnd().split("\t")
      // fields[0] = error code (0 = ok, already validated by requireOk inside execute)
      val nRep   = fields.getOrNull(1)?.trim() ?: ""
      val totA   = fields.getOrNull(2)?.trim() ?: ""
      val result = mapOf("state" to "FISCALIZED", "n_rep" to nRep, "tot_a" to totA,
                         "report_type" to reportType)
      journal.append(operationId, "FISCALIZED", encode(result))
      result
    } catch (e: Throwable) {
      // ERR_Z_REPORT_CLOSED = -104006: Z was already done on the FU → idempotent success.
      if (e is DatecsFailure.Rejected && e.keyCode == -104006) {
        val result = mapOf("state" to "FISCALIZED", "error_code" to "Z_ALREADY_CLOSED",
                           "report_type" to reportType)
        journal.append(operationId, "FISCALIZED", encode(result))
        return result
      }
      val unknown = classified(e) == Outcome.UNKNOWN
      val result = mapOf("state" to if (unknown) "UNKNOWN" else "FAILED",
                         "error_code" to (e.message ?: "DATECS_FAILURE"))
      journal.append(operationId, result.getValue("state"), encode(result))
      result
    }
  }

  fun printerTest(
    operationId: String,
    acceptedEvidence: String = "state=ACCEPTED"
  ): Map<String, String> {
    require(operationId.isNotBlank())
    return lock.withLock {
    val old = journal.find(operationId)
    old
      .firstOrNull { it.type == "ACCEPTED" }
      ?.let { require(it.payload == acceptedEvidence) { "COMMAND_ID_PAYLOAD_CONFLICT" } }
    old
      .lastOrNull { it.type in setOf("PRINTER_TESTED", "FAILED", "UNKNOWN") }
      ?.let {
        return@withLock parse(it.payload)
      }
    require(fiscal.reachable()) { "FISCAL_DEVICE_UNREACHABLE" }
    journal.append(operationId, "ACCEPTED", acceptedEvidence)
    journal.append(operationId, "EXECUTING", "state=EXECUTING")
    var receiptOpened = false
    try {
      ok(fiscal.execute(38))
      receiptOpened = true
      ok(fiscal.execute(42, DatecsText.encode("BEELOY PRINTER TEST\t\t\t\t\t\t")))
      ok(fiscal.execute(39))
      mapOf("state" to "PRINTER_TESTED").also {
        journal.append(operationId, "PRINTER_TESTED", encode(it))
      }
    } catch (e: Throwable) {
      val unknown = classified(e) == Outcome.UNKNOWN
      if (receiptOpened && !unknown) {
        val closeOk = runCatching { fiscal.execute(39) }.isSuccess
        if (closeOk) {
          journal.append(operationId, "RECEIPT_CANCELLED", "state=RECEIPT_CANCELLED")
        } else {
          journal.append(operationId, "RECEIPT_CANCEL_FAILED", "state=RECEIPT_CANCEL_FAILED")
          val result = mapOf("state" to "RECOVERY_REQUIRED", "error_code" to "RECEIPT_CANCEL_FAILED")
          journal.append(operationId, "RECOVERY_REQUIRED", encode(result))
          return result
        }
      }
      val result =
        mapOf(
          "state" to if (unknown) "UNKNOWN" else "FAILED",
          "error_code" to (e.message ?: "PRINTER_TEST_FAILURE")
        )
      journal.append(operationId, result.getValue("state"), encode(result))
      result
    }
    }
  }
  fun sale(
    command: BlueCashSale,
    acceptedEvidence: String = "state=ACCEPTED"
  ): Map<String, String> {
    require(
      command.operationId.isNotBlank() &&
        command.lines.isNotEmpty() &&
        command.payments.isNotEmpty()
    )
    return lock.withLock {
    val old = journal.find(command.operationId)
    old
      .firstOrNull { it.type == "ACCEPTED" }
      ?.let { require(it.payload == acceptedEvidence) { "COMMAND_ID_PAYLOAD_CONFLICT" } }
    old
      .lastOrNull { it.type in SALE_TERMINAL }
      ?.let {
        return@withLock parse(it.payload)
      }
    if (old.any { it.type == "ACCEPTED" || it.type == "EXECUTING" })
      return@withLock mapOf("state" to "UNKNOWN", "error_code" to "RECOVERY_REQUIRED")
    val payloadErrors = DatecsPayloads.validateSale(command.lines, command.payments)
    if (payloadErrors.isNotEmpty()) {
      return@withLock mapOf("state" to "FAILED", "error_code" to payloadErrors.first())
    }
    require(fiscal.reachable()) { "FISCAL_DEVICE_UNREACHABLE" }
    journal.append(command.operationId, "ACCEPTED", acceptedEvidence)
    journal.append(command.operationId, "EXECUTING", "state=EXECUTING")
    val approvedCardPayments = mutableListOf<SalePayment>()
    var receiptOpened = false
    try {
      for (p in command.payments.filter { it.type == "CARD" }) {
        require(card.reachable()) { "PAYMENT_TERMINAL_UNREACHABLE" }
        journal.append(
          command.operationId,
          "PAYMENT_PREPARED",
          "payment_id=${p.id}&amount=${p.amount}"
        )
        val payment = card.purchaseEur(moneyMinor(p.amount), p.id)
        require(payment["approved"] == "true") { payment["error_code"] ?: "CARD_DECLINED" }
        approvedCardPayments += p
        journal.append(
          command.operationId,
          "PAYMENT_APPROVED",
          "payment_id=${p.id}&rrn=${payment["rrn"].orEmpty()}&authorization_code=${payment["authorization_code"].orEmpty()}"
        )
      }
      ok(
        fiscal.execute(
          48,
          DatecsPayloads.open(command.operator, command.password, command.unp, command.till)
        )
      )
      receiptOpened = true
      command.lines.forEach { ok(fiscal.execute(49, DatecsPayloads.line(it))) }
      command.payments.forEach { ok(fiscal.execute(53, DatecsPayloads.payment(datecsPaymentMode(it.type), it.amount))) }
      val closed = ok(fiscal.execute(56))
      val ref = DatecsText.decode(closed.data).split('\t').getOrNull(1)?.takeIf { it.isNotBlank() }
      if (ref == null) {
        val result = mapOf("state" to "RECOVERY_REQUIRED", "error_code" to "FISCAL_REFERENCE_UNPARSEABLE")
        journal.append(command.operationId, "RECOVERY_REQUIRED", encode(result))
        return result
      }
      mapOf("state" to "FISCALIZED", "fiscal_reference" to ref).also {
        journal.append(command.operationId, "FISCALIZED", encode(it))
      }
    } catch (e: Throwable) {
      val unknown = classified(e) == Outcome.UNKNOWN
      if (receiptOpened && !unknown) {
        val cancelOk = runCatching { fiscal.execute(DatecsPayloads.CMD_CANCEL_RECEIPT) }.isSuccess
        if (cancelOk) {
          journal.append(command.operationId, "RECEIPT_CANCELLED", "state=RECEIPT_CANCELLED")
        } else {
          journal.append(command.operationId, "RECEIPT_CANCEL_FAILED", "state=RECEIPT_CANCEL_FAILED")
          val result = mapOf("state" to "RECOVERY_REQUIRED", "error_code" to "RECEIPT_CANCEL_FAILED")
          journal.append(command.operationId, "RECOVERY_REQUIRED", encode(result))
          return result
        }
      }
      if (approvedCardPayments.isNotEmpty() && !unknown) {
        // Fiscal definitively failed: safe to reverse the card (D-09).
        journal.append(
          command.operationId,
          "COMPENSATION_REQUIRED",
          "approved_payment_ids=${approvedCardPayments.joinToString(","){it.id}}"
        )
        val reversed =
          approvedCardPayments.asReversed().all { p ->
            runCatching { card.reverse(p.id)["approved"] == "true" }.getOrDefault(false)
          }
        val result =
          if (reversed)
            mapOf(
              "state" to "COMPENSATED",
              "error_code" to (e.message ?: "FISCAL_FAILURE_AFTER_CARD")
            )
          else mapOf("state" to "RECOVERY_REQUIRED", "error_code" to "CARD_COMPENSATION_FAILED")
        journal.append(command.operationId, result.getValue("state"), encode(result))
        return result
      }
      if (approvedCardPayments.isNotEmpty()) {
        // unknown=true: fiscal outcome unclear — do NOT reverse card (invariant 2).
        val result = mapOf("state" to "RECOVERY_REQUIRED", "error_code" to "FISCAL_RESULT_UNKNOWN")
        journal.append(command.operationId, "RECOVERY_REQUIRED", encode(result))
        return result
      }
      val result =
        mapOf(
          "state" to if (unknown) "UNKNOWN" else "FAILED",
          "error_code" to (e.message ?: "DATECS_FAILURE")
        )
      journal.append(command.operationId, result.getValue("state"), encode(result))
      result
    }
    }
  }
  fun reverse(
    command: BlueCashReversal,
    acceptedEvidence: String = "state=ACCEPTED"
  ): Map<String, String> {
    require(
      command.operationId.isNotBlank() &&
        command.originalOperationId.isNotBlank() &&
        command.lines.isNotEmpty() &&
        command.payments.isNotEmpty()
    )
    return lock.withLock {
    val old = journal.find(command.operationId)
    old
      .firstOrNull { it.type == "ACCEPTED" }
      ?.let { require(it.payload == acceptedEvidence) { "COMMAND_ID_PAYLOAD_CONFLICT" } }
    old
      .lastOrNull { it.type in setOf("REVERSED", "FAILED", "UNKNOWN", "REFUND_PENDING") }
      ?.let {
        return@withLock parse(it.payload)
      }
    if (old.any { it.type == "ACCEPTED" || it.type == "EXECUTING" })
      return@withLock mapOf("state" to "UNKNOWN", "error_code" to "RECOVERY_REQUIRED")
    require(fiscal.reachable()) { "FISCAL_DEVICE_UNREACHABLE" }
    journal.append(command.operationId, "ACCEPTED", acceptedEvidence)
    journal.append(command.operationId, "EXECUTING", "state=EXECUTING")
    var receiptOpened = false
    try {
      // Storno-first (invariant 3): fiscal reversal before any card refund.
      ok(
        fiscal.execute(
          43,
          DatecsPayloads.stornoOpen(
            command.operator,
            command.password,
            command.till,
            command.originalDocument
          )
        )
      )
      receiptOpened = true
      command.lines.forEach { ok(fiscal.execute(49, DatecsPayloads.line(it))) }
      command.payments.forEach { ok(fiscal.execute(53, DatecsPayloads.payment(datecsPaymentMode(it.type), it.amount))) }
      val closed = ok(fiscal.execute(56))
      val ref = DatecsText.decode(closed.data).split('\t').getOrNull(1)?.takeIf { it.isNotBlank() }
      if (ref == null) {
        val result = mapOf("state" to "RECOVERY_REQUIRED", "error_code" to "FISCAL_REFERENCE_UNPARSEABLE")
        journal.append(command.operationId, "RECOVERY_REQUIRED", encode(result))
        return result
      }
      // Fiscal storno confirmed: safe to refund card.
      for (p in command.payments.filter { it.type == "CARD" }.asReversed()) {
        require(card.reachable()) { "PAYMENT_TERMINAL_UNREACHABLE" }
        journal.append(
          command.operationId,
          "REFUND_PREPARED",
          "payment_id=${p.id}&amount=${p.amount}&original_operation_id=${command.originalOperationId}"
        )
        val payment = runCatching { card.reverse(p.id) }.getOrElse { mapOf("approved" to "false") }
        if (payment["approved"] != "true") {
          val result = mapOf(
            "state" to "REFUND_PENDING",
            "fiscal_reference" to ref,
            "error_code" to (payment["error_code"] ?: "CARD_REVERSAL_DECLINED")
          )
          journal.append(command.operationId, "REFUND_PENDING", encode(result))
          return result
        }
        journal.append(
          command.operationId,
          "REFUND_APPROVED",
          "payment_id=${p.id}&rrn=${payment["rrn"].orEmpty()}&authorization_code=${payment["authorization_code"].orEmpty()}"
        )
      }
      mapOf("state" to "REVERSED", "fiscal_reference" to ref).also {
        journal.append(command.operationId, "REVERSED", encode(it))
      }
    } catch (e: Throwable) {
      val unknown = classified(e) == Outcome.UNKNOWN
      if (receiptOpened && !unknown) {
        val cancelOk = runCatching { fiscal.execute(DatecsPayloads.CMD_CANCEL_RECEIPT) }.isSuccess
        if (cancelOk) {
          journal.append(command.operationId, "RECEIPT_CANCELLED", "state=RECEIPT_CANCELLED")
        } else {
          journal.append(command.operationId, "RECEIPT_CANCEL_FAILED", "state=RECEIPT_CANCEL_FAILED")
          val result = mapOf("state" to "RECOVERY_REQUIRED", "error_code" to "RECEIPT_CANCEL_FAILED")
          journal.append(command.operationId, "RECOVERY_REQUIRED", encode(result))
          return result
        }
      }
      val result =
        mapOf(
          "state" to if (unknown) "UNKNOWN" else "FAILED",
          "error_code" to (e.message ?: "DATECS_REVERSAL_FAILURE")
        )
      journal.append(command.operationId, result.getValue("state"), encode(result))
      result
    }
    }
  }
  private fun ok(r: DatecsResponse): DatecsResponse {
    val text = DatecsText.decode(r.data)
    require(text.startsWith("0\t") || text == "0" || text.isEmpty()) {
      text.substringBefore('\t').ifBlank { "DATECS_REJECTED" }
    }
    return r
  }
  private fun moneyMinor(v: String): Long {
    require(v.matches(Regex("[0-9]+\\.[0-9]{2}")))
    return v.replace(".", "").toLong()
  }
  private fun encode(v: Map<String, String>) =
    v.entries.sortedBy { it.key }.joinToString("&") { "${it.key}=${it.value}" }
  private fun parse(v: String) =
    v.split('&').associate {
      val p = it.split('=', limit = 2)
      p[0] to p.getOrElse(1) { "" }
    }
  /** BUG-028: typed error classification — replaces text-based checks with `when(e)`. */
  private fun classified(e: Throwable): Outcome =
    when (e) {
      is DatecsFailure.Unknown -> Outcome.UNKNOWN
      is DatecsFailure.Rejected -> Outcome.FAILED
      is DatecsFailure.NotSent -> Outcome.FAILED
      is DatecsFailure.CorrelationError -> Outcome.FAILED
      is DatecsFailure.BadFrame -> Outcome.FAILED
      is DatecsFailure.BccError -> Outcome.FAILED
      is DatecsFailure.FrameTooLarge -> Outcome.FAILED
      else -> Outcome.FAILED
    }
}

/** Outcome of a Datecs fiscal operation (BUG-028 typed classification). */
sealed class Outcome {
  object UNKNOWN : Outcome()
  object FAILED : Outcome()
}

/**
 * VG-08.1: Map a POS reason code to the Datecs storno reason integer (§4.6).
 *   0 = OPERATOR_ERROR  (operator mistake)
 *   1 = CUSTOMER_RETURN (customer return / refund)
 *   2 = TAX_BASE_REDUCTION
 */
fun stornoReasonCode(posReason: String): Int = when (posReason.uppercase()) {
  "OPERATOR_ERROR"         -> 0
  "CUSTOMER_RETURN"        -> 1
  "TAX_BASE_REDUCTION"     -> 2
  else -> throw IllegalArgumentException("UNKNOWN_STORNO_REASON:$posReason")
}

internal fun datecsPaymentMode(type: String): Int =
  when (type) {
    "CASH" -> 0
    "CARD" -> 1
    "CHEQUE" -> 2
    "VOUCHER", "COUPON" -> 3
    "NHIF" -> 4
    "DEFERRED", "INTERNAL_CONSUMPTION" -> 5
    else -> error("PAYMENT_TYPE")
  }

/**
 * VG-23: Hardware device profiles for payment slot mapping.
 */
enum class BC50Profile {
    DEFAULT
}

/**
 * VG-23: Map a POS payment type to a Datecs paid-mode slot for a given device profile.
 * BC50_DEFAULT from PM §4.15 and docs/MVP1/fixtures/payment_slots_bc50.json:
 *   CASH→0, CARD→1, CHEQUE→3, VOUCHER→4, COUPON→5.
 * NHIF, DEFERRED, INTERNAL_CONSUMPTION → null (PAYMENT_TYPE_UNSUPPORTED).
 * Validation must happen before any card or fiscal I/O (invariant: BUG-022).
 */
fun paymentSlot(profile: BC50Profile, type: String): Int? =
    when (profile) {
        BC50Profile.DEFAULT -> when (type) {
            "CASH"   -> 0
            "CARD"   -> 1
            "CHEQUE" -> 3
            "VOUCHER" -> 4
            "COUPON"  -> 5
            "NHIF", "DEFERRED", "INTERNAL_CONSUMPTION" -> null
            else -> error("PAYMENT_TYPE_UNKNOWN:$type")
        }
    }
