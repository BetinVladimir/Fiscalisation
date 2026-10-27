package com.beeloy.fiscal.bluecash

import java.time.Instant
import org.junit.Assert.*
import org.junit.Test

class Signer : TransactionSigner {
  override val keyId = "test"
  override fun sign(hash: ByteArray) = hash
}

class Fiscal : DatecsFiscalPort {
  val calls = mutableListOf<Pair<Int, String>>()
  var fail = false
  override fun reachable() = true
  override fun execute(command: Int, payload: ByteArray): DatecsResponse {
    calls += command to String(payload)
    if (fail) error("DATECS_EOF")
    return DatecsResponse(
      32,
      command,
      if (command == 56) "0\tBON-42\t".toByteArray() else "0\t".toByteArray(),
      byteArrayOf()
    )
  }
}

class Card : DatecsPaymentPort {
  var calls = 0
  var reversals = 0
  var reversedOperation: String? = null
  val reversedOperations = mutableListOf<String>()
  override fun reachable() = true
  override fun purchaseEur(amountMinor: Long, operationId: String) =
    mapOf("approved" to "true", "rrn" to "1").also { calls++ }
  override fun reverse(operationId: String) =
    mapOf("approved" to "true").also {
      reversals++
      reversedOperation = operationId
      reversedOperations += operationId
    }
}

class BlueCashEngineTest {
  private fun sale() =
    BlueCashSale(
      "op-1",
      "DY000600-OP01-0000001",
      1,
      "1",
      24,
      listOf(FiscalLine("Coffee", 'B', "2.65", "1.000")),
      listOf(SalePayment("p", "CARD", "2.65"))
    )
  @Test
  fun `card and fiscal receipt execute once in documented order`() {
    val f = Fiscal()
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(f, c, j)
    assertEquals("FISCALIZED", p.sale(sale())["state"])
    assertEquals(listOf(48, 49, 53, 56), f.calls.map { it.first })
    assertEquals(1, c.calls)
    assertEquals("FISCALIZED", p.sale(sale())["state"])
    assertEquals(1, c.calls)
    assertEquals(
      listOf("ACCEPTED", "EXECUTING", "PAYMENT_PREPARED", "PAYMENT_APPROVED", "FISCALIZED"),
      j.find("op-1").map { it.type }
    )
  }
  @Test
  fun `fiscal failure after approved card is compensated by payment id`() {
    val f = Fiscal().apply { fail = true }
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(f, c, j).sale(sale())
    assertEquals("COMPENSATED", result["state"])
    assertEquals("p", c.reversedOperation)
    assertEquals(
      listOf(
        "ACCEPTED",
        "EXECUTING",
        "PAYMENT_PREPARED",
        "PAYMENT_APPROVED",
        "COMPENSATION_REQUIRED",
        "COMPENSATED"
      ),
      j.find("op-1").map { it.type }
    )
  }
  @Test
  fun `restart after executing fails closed without repeating device side effect`() {
    val f = Fiscal()
    val j = SignedMemoryJournal(Signer())
    j.append("op-1", "ACCEPTED", "state=ACCEPTED")
    j.append("op-1", "EXECUTING", "state=EXECUTING")
    val result = BlueCashCommandProcessor(f, Card(), j).sale(sale())
    assertEquals("UNKNOWN", result["state"])
    assertEquals("RECOVERY_REQUIRED", result["error_code"])
    assertTrue(f.calls.isEmpty())
  }
  @Test
  fun `same operation id with different accepted payload is rejected`() {
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(Fiscal(), Card(), j)
    p.sale(sale(), "payload-a")
    assertThrows(IllegalArgumentException::class.java) { p.sale(sale(), "payload-b") }
  }
  @Test
  fun `card reversal and Datecs storno execute once in documented order`() {
    val f = Fiscal()
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(f, c, j)
    val d = DatecsStornoDocument(1, 42, "11-08-26 12:30:45", "12345678", "DY000600-OP01-0000001")
    val cmd = BlueCashReversal("rev-1", "op-1", 1, "1", 24, d, sale().lines, sale().payments)
    assertEquals("REVERSED", p.reverse(cmd)["state"])
    assertEquals(listOf(43, 49, 53, 56), f.calls.map { it.first })
    assertEquals("p", c.reversedOperation)
    assertEquals("REVERSED", p.reverse(cmd)["state"])
    assertEquals(1, c.reversals)
    assertEquals(
      listOf("ACCEPTED", "EXECUTING", "REFUND_PREPARED", "REFUND_APPROVED", "REVERSED"),
      j.find("rev-1").map { it.type }
    )
  }
  @Test
  fun `multiple card reversals use each original payment id in reverse order`() {
    val f = Fiscal()
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(f, c, j)
    val d = DatecsStornoDocument(1, 42, "11-08-26 12:30:45", "12345678", "DY000600-OP01-0000001")
    val payments = listOf(SalePayment("p1", "CARD", "1.00"), SalePayment("p2", "CARD", "1.65"))
    val cmd = BlueCashReversal("rev-2", "op-1", 1, "1", 24, d, sale().lines, payments)
    assertEquals("REVERSED", p.reverse(cmd)["state"])
    assertEquals(listOf("p2", "p1"), c.reversedOperations)
  }
  @Test
  fun `printer test is non fiscal and idempotent`() {
    val f = Fiscal()
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(f, c, j)
    assertEquals("PRINTER_TESTED", p.printerTest("print-1")["state"])
    assertEquals(listOf(38, 42, 39), f.calls.map { it.first })
    assertEquals("PRINTER_TESTED", p.printerTest("print-1")["state"])
    assertEquals(3, f.calls.size)
    assertEquals(0, c.calls)
  }
  @Test
  fun `printerTestClosesNonFiscalReceiptOnFailure`() {
    val calls = mutableListOf<Int>()
    val failOn42 = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse {
        calls += command
        if (command == 42) error("DATECS_LINE_FAIL")
        return DatecsResponse(32, command, "0\t".toByteArray(), byteArrayOf())
      }
    }
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(failOn42, Card(), j)
    val result = p.printerTest("pt-fail-1")
    assertEquals("FAILED", result["state"])
    assertEquals("cmd 39 must close non-fiscal doc after cmd 42 fail",
      listOf(38, 42, 39), calls)
  }
  @Test
  fun `reports and cash movements are durable and idempotent`() {
    val f = Fiscal()
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(f, Card(), j)
    assertEquals("FISCALIZED", p.administrative("x-1", 69, DatecsPayloads.report(false))["state"])
    assertEquals(
      "FISCALIZED",
      p.administrative("cash-1", 70, DatecsPayloads.cash(false, "10.00"))["state"]
    )
    assertEquals(
      "FISCALIZED",
      p.administrative("cash-1", 70, DatecsPayloads.cash(false, "10.00"))["state"]
    )
    assertEquals(listOf(69, 70), f.calls.map { it.first })
  }
  @Test
  fun `retention removes only business acknowledged old records and keeps anchor`() {
    var now = Instant.parse("2026-01-01T00:00:00Z")
    val j = SignedMemoryJournal(Signer()) { now }
    j.append("op", "ACCEPTED", "state=ACCEPTED")
    val anchor = j.append("op", "FISCALIZED", "state=FISCALIZED")
    now = Instant.parse("2026-05-01T00:00:00Z")
    assertEquals(0, j.purgeBefore(now.minusSeconds(90L * 86400)))
    j.acknowledge(anchor.sequence, anchor.hash, "ack")
    assertEquals(anchor.hash, j.checkpoint().second)
    assertEquals(1, j.purgeBefore(now.minusSeconds(90L * 86400)))
    assertEquals(listOf(anchor.sequence), j.find("op").map { it.sequence })
  }
  @Test
  fun `bluecash protocol faithful fault profile covers decline timeout restart duplicate compensation and ack loss`() {
    val approvedFiscal = Fiscal()
    val journal = SignedMemoryJournal(Signer())
    val processor = BlueCashCommandProcessor(approvedFiscal, Card(), journal)
    assertEquals("FISCALIZED", processor.sale(sale())["state"])
    assertEquals(1, approvedFiscal.calls.count { it.first == 48 })
    assertEquals("FISCALIZED", processor.sale(sale())["state"])
    assertEquals(1, approvedFiscal.calls.count { it.first == 48 })
    val beforeAck = journal.pending()
    assertTrue(beforeAck.isNotEmpty())
    assertEquals(beforeAck, journal.pending())
    val declined =
      object : DatecsPaymentPort {
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String) =
          mapOf("approved" to "false", "error_code" to "CARD_DECLINED")
        override fun reverse(operationId: String) = mapOf("approved" to "false")
      }
    val declineSale = sale().copy(operationId = "op-decline")
    assertEquals(
      "FAILED",
      BlueCashCommandProcessor(Fiscal(), declined, SignedMemoryJournal(Signer()))
        .sale(declineSale)["state"]
    )
    val timeoutFiscal = Fiscal().apply { fail = true }
    assertEquals(
      "COMPENSATED",
      BlueCashCommandProcessor(timeoutFiscal, Card(), SignedMemoryJournal(Signer()))
        .sale(sale().copy(operationId = "op-timeout"))["state"]
    )
    val restartJournal = SignedMemoryJournal(Signer())
    restartJournal.append("op-restart", "ACCEPTED", "state=ACCEPTED")
    restartJournal.append("op-restart", "EXECUTING", "state=EXECUTING")
    val restartFiscal = Fiscal()
    assertEquals(
      "UNKNOWN",
      BlueCashCommandProcessor(restartFiscal, Card(), restartJournal)
        .sale(sale().copy(operationId = "op-restart"))["state"]
    )
    assertTrue(restartFiscal.calls.isEmpty())
  }

  // BUG-026: document number must come from FU response, not a synthetic prefix.
  @Test
  fun closeReceiptWithoutParsableDocumentNumberIsUnknown() {
    val emptyFiscal = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse =
        // cmd56 (close receipt): success status but blank doc-number field → no ref.
        if (command == 56) DatecsResponse(0x20, command, "0\t \t".toByteArray(), byteArrayOf())
        else DatecsResponse(0x20, command, "0\t".toByteArray(), byteArrayOf())
    }
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(emptyFiscal, Card(), j).sale(sale())
    // Without a parseable reference the saga must signal recovery, not invent one.
    assertEquals("RECOVERY_REQUIRED", result["state"])
    assertEquals("FISCAL_REFERENCE_UNPARSEABLE", result["error_code"])
    assertNull(result["fiscal_reference"])
  }

  @Test
  fun closeReceiptParsesDocumentNumberFromVendorFixture() {
    // Response matches PM v2.11.4 §4 cmd56 example: tab-delimited "0\tBON-42\t".
    val vendorFiscal = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse =
        DatecsResponse(0x20, command,
          if (command == 56) "0\tBON-42\t".toByteArray(Charsets.UTF_8) else "0\t".toByteArray(),
          byteArrayOf())
    }
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(vendorFiscal, Card(), j).sale(sale())
    assertEquals("FISCALIZED", result["state"])
    assertEquals("BON-42", result["fiscal_reference"])
  }

  // BUG-026.3: acceptance names from spec.
  @Test
  fun saleResultCarriesRealDocumentNumber() {
    val j = SignedMemoryJournal(Signer())
    // Default Fiscal mock returns "0\tBON-42\t" for cmd56.
    val result = BlueCashCommandProcessor(Fiscal(), Card(), j).sale(sale())
    assertEquals("FISCALIZED", result["state"])
    assertEquals("BON-42", result["fiscal_reference"])
  }

  // BUG-020.5: open-receipt cancellation on failure ────────────────────────────

  /** Configurable fake port: specific commands throw the given exception. */
  private class FailAtPort(private val failMap: Map<Int, Throwable>) : DatecsFiscalPort {
    val calls = mutableListOf<Int>()
    override fun reachable() = true
    override fun execute(command: Int, payload: ByteArray): DatecsResponse {
      calls += command
      failMap[command]?.let { throw it }
      return DatecsResponse(0x20, command,
        if (command == 56) "0\tBON-42\t".toByteArray() else "0\t".toByteArray(),
        byteArrayOf())
    }
  }

  private fun cashSale(operationId: String = "op-1") = BlueCashSale(
    operationId, "DY000600-OP01-0000001", 1, "1", 24,
    listOf(FiscalLine("Coffee", 'B', "2.65", "1.000")),
    listOf(SalePayment("p", "CASH", "2.65"))
  )

  @Test
  fun saleCancelsOpenReceiptWhenLineRejected() {
    val f = FailAtPort(mapOf(49 to DatecsFailure.Rejected(22, "-111")))
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(f, Card(), j).sale(cashSale())
    assertEquals(listOf(48, 49, DatecsPayloads.CMD_CANCEL_RECEIPT), f.calls)
    assertEquals("FAILED", result["state"])
    assertEquals("-111", result["error_code"])
    assertTrue(j.find("op-1").any { it.type == "RECEIPT_CANCELLED" })
  }

  @Test
  fun saleCancelsOpenReceiptWhenPaymentRejected() {
    val f = FailAtPort(mapOf(53 to DatecsFailure.Rejected(22, "-112")))
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(f, Card(), j).sale(cashSale())
    assertEquals(listOf(48, 49, 53, DatecsPayloads.CMD_CANCEL_RECEIPT), f.calls)
    assertEquals("FAILED", result["state"])
    assertEquals("-112", result["error_code"])
    assertTrue(j.find("op-1").any { it.type == "RECEIPT_CANCELLED" })
  }

  @Test
  fun saleDoesNotBlindCancelOnUnknown() {
    // EOF after payment (cmd 53) → Unknown → no cancel, no card reversal.
    val f = FailAtPort(mapOf(53 to DatecsFailure.Unknown("DATECS_EOF")))
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(f, Card(), j).sale(cashSale())
    assertEquals(listOf(48, 49, 53), f.calls)
    assertEquals("UNKNOWN", result["state"])
    assertFalse(j.find("op-1").any { it.type == "RECEIPT_CANCELLED" })
  }

  @Test
  fun cancelFailureIsJournaled() {
    // Line rejected AND cancel also fails → RECOVERY_REQUIRED, RECEIPT_CANCEL_FAILED in journal.
    val f = FailAtPort(mapOf(
      49 to DatecsFailure.Rejected(22, "-111"),
      DatecsPayloads.CMD_CANCEL_RECEIPT to DatecsFailure.Unknown("DATECS_EOF")
    ))
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(f, Card(), j).sale(cashSale())
    assertEquals("RECOVERY_REQUIRED", result["state"])
    assertTrue(j.find("op-1").any { it.type == "RECEIPT_CANCEL_FAILED" })
    assertFalse(j.find("op-1").any { it.type == "RECEIPT_CANCELLED" })
  }

  // BUG-071: storno-first (card refund only after fiscal storno confirmed) ──────

  @Test
  fun reverseCardRefundNeverBeforeFiscalStorno() {
    val log = mutableListOf<String>()
    val f = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse {
        log += "fiscal:$command"
        return DatecsResponse(0x20, command, if (command == 56) "0\tBON-99\t".toByteArray() else "0\t".toByteArray(), byteArrayOf())
      }
    }
    val c = object : DatecsPaymentPort {
      override fun reachable() = true
      override fun purchaseEur(amountMinor: Long, operationId: String) = mapOf("approved" to "true")
      override fun reverse(operationId: String) = mapOf("approved" to "true").also { log += "card:reverse" }
    }
    val j = SignedMemoryJournal(Signer())
    val d = DatecsStornoDocument(1, 42, "11-08-26 12:30:45", "12345678", "DY000600-OP01-0000001")
    val cmd = BlueCashReversal("rev-x", "op-1", 1, "1", 24, d,
      listOf(FiscalLine("Coffee", 'B', "2.65", "1.000")),
      listOf(SalePayment("p", "CARD", "2.65")))
    val result = BlueCashCommandProcessor(f, c, j).reverse(cmd)
    assertEquals("REVERSED", result["state"])
    val cardIdx = log.indexOfFirst { it == "card:reverse" }
    val lastFiscalIdx = log.indexOfLast { it.startsWith("fiscal:") }
    assertTrue("card:reverse (idx=$cardIdx) must come after last fiscal call (idx=$lastFiscalIdx)", cardIdx > lastFiscalIdx)
  }

  @Test
  fun reverseStornoUnknownDoesNotRefund() {
    val f = FailAtPort(mapOf(43 to DatecsFailure.Unknown("DATECS_EOF")))
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val d = DatecsStornoDocument(1, 42, "11-08-26 12:30:45", "12345678", "DY000600-OP01-0000001")
    val cmd = BlueCashReversal("rev-u", "op-1", 1, "1", 24, d,
      listOf(FiscalLine("Coffee", 'B', "2.65", "1.000")),
      listOf(SalePayment("p", "CARD", "2.65")))
    val result = BlueCashCommandProcessor(f, c, j).reverse(cmd)
    assertEquals("UNKNOWN", result["state"])
    assertEquals(0, c.reversals)
  }

  @Test
  fun reverseRefundFailureAfterStornoIsRetryable() {
    val f = Fiscal()
    val noRefundCard = object : DatecsPaymentPort {
      override fun reachable() = true
      override fun purchaseEur(amountMinor: Long, operationId: String) = mapOf("approved" to "true")
      override fun reverse(operationId: String) = mapOf("approved" to "false", "error_code" to "CARD_NETWORK_TIMEOUT")
    }
    val j = SignedMemoryJournal(Signer())
    val d = DatecsStornoDocument(1, 42, "11-08-26 12:30:45", "12345678", "DY000600-OP01-0000001")
    val cmd = BlueCashReversal("rev-r", "op-1", 1, "1", 24, d,
      listOf(FiscalLine("Coffee", 'B', "2.65", "1.000")),
      listOf(SalePayment("p", "CARD", "2.65")))
    val result = BlueCashCommandProcessor(f, noRefundCard, j).reverse(cmd)
    assertEquals("REFUND_PENDING", result["state"])
    assertNotNull(result["fiscal_reference"])
  }

  // BUG-021: no card reversal when fiscal result is Unknown ────────────────────

  @Test
  fun saleDoesNotReverseCardWhenFiscalResultUnknown() {
    // cmd 56 (close) throws Unknown (DATECS_EOF): fiscal outcome unclear → no card reversal
    val f = FailAtPort(mapOf(56 to DatecsFailure.Unknown("DATECS_EOF")))
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(f, c, j).sale(sale())
    assertEquals(0, c.reversals)
    assertEquals("RECOVERY_REQUIRED", result["state"])
  }

  @Test
  fun saleReversesCardWhenFiscalDefinitelyRejected() {
    // cmd 56 definitively rejected → fiscal not fiscalized → safe to reverse card
    val f = FailAtPort(mapOf(56 to DatecsFailure.Rejected(22, "FISCAL_REJECTED")))
    val c = Card()
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(f, c, j).sale(sale())
    assertEquals(1, c.reversals)
    assertEquals("COMPENSATED", result["state"])
  }

  // VG-01: fiscal device expects CP1251, not UTF-8 ─────────────────────────────

  @Test
  fun linePayloadEncodesCyrillicAsWindows1251() {
    // "Кафе" in CP1251: К=0xCA, а=0xE0, ф=0xF4, е=0xE5 (1 byte each)
    // In UTF-8 "К" = 0xD0 0x9A — two bytes; first byte 0xD0 ≠ 0xCA → fails currently
    val line = FiscalLine("Кафе", 'B', "2.65", "1.000")
    val encoded = DatecsPayloads.line(line)
    assertEquals(0xCA.toByte(), encoded[0])
    assertEquals(0xE0.toByte(), encoded[1])
    assertEquals(0xF4.toByte(), encoded[2])
    assertEquals(0xE5.toByte(), encoded[3])
    assertEquals('\t'.code.toByte(), encoded[4])
  }

  @Test
  fun fiscalReferenceDecodedFromWindows1251() {
    // FU returns "0\tБОН-42\t" in CP1251 bytes; ok() must decode as CP1251, not UTF-8
    val cp1251 = java.nio.charset.Charset.forName("windows-1251")
    val responseBytes = "0\tБОН-42\t".toByteArray(cp1251)
    val fakePort = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse =
        DatecsResponse(0x20, command, if (command == 56) responseBytes else "0\t".toByteArray(cp1251), byteArrayOf())
    }
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(fakePort, Card(), j).sale(cashSale())
    assertEquals("FISCALIZED", result["state"])
    assertEquals("БОН-42", result["fiscal_reference"])
  }

  // ── BUG-025: COMPENSATED and RECOVERY_REQUIRED must be terminal idempotency states ──

  @Test fun `replayOfCompensatedSaleReturnsCompensated`() {
    val f = Fiscal().apply { fail = true }
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(f, Card(), j)
    val first = p.sale(sale())
    assertEquals("COMPENSATED", first["state"])
    // Replay with same operationId must return COMPENSATED, not try again.
    val second = p.sale(sale())
    assertEquals("replay must return COMPENSATED", "COMPENSATED", second["state"])
  }

  @Test fun `replayOfRecoveryRequiredReturnsRecoveryRequired`() {
    // After a timeout on cmd56, state is RECOVERY_REQUIRED.
    val blankFiscal = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse =
        if (command == 56) DatecsResponse(0x20, command, "0\t\t".toByteArray(), byteArrayOf())
        else DatecsResponse(0x20, command, "0\t".toByteArray(), byteArrayOf())
    }
    val j = SignedMemoryJournal(Signer())
    val p = BlueCashCommandProcessor(blankFiscal, Card(), j)
    val first = p.sale(sale())
    assertEquals("RECOVERY_REQUIRED", first["state"])
    val second = p.sale(sale())
    assertEquals("replay must return RECOVERY_REQUIRED", "RECOVERY_REQUIRED", second["state"])
  }

  // ── BUG-022: payload validation must happen before card purchase ─────────────

  @Test fun `salePrevalidatesAllPayloadsBeforeCardPurchase`() {
    val c = Card()
    val badSale = BlueCashSale(
      "op-pre-1", "DY000600-OP01-0000001", 1, "1", 24,
      listOf(FiscalLine("A".repeat(73), 'B', "2.65", "1.000")),
      listOf(SalePayment("p", "CARD", "2.65"))
    )
    val result = BlueCashCommandProcessor(Fiscal(), c, SignedMemoryJournal(Signer())).sale(badSale)
    assertEquals("FAILED", result["state"])
    assertTrue("error_code must mention PAYLOAD_INVALID",
      result["error_code"]?.startsWith("PAYLOAD_INVALID") == true)
    assertEquals("card.purchaseEur must NOT be called before validation", 0, c.calls)
  }

  @Test fun `quantityScientificNotationRejected`() {
    val c = Card()
    val badSale = BlueCashSale(
      "op-sci-1", "DY000600-OP01-0000001", 1, "1", 24,
      listOf(FiscalLine("Coffee", 'B', "2.65", "1e3")),
      listOf(SalePayment("p", "CARD", "2.65"))
    )
    val result = BlueCashCommandProcessor(Fiscal(), c, SignedMemoryJournal(Signer())).sale(badSale)
    assertEquals("FAILED", result["state"])
    assertEquals("card must not be charged for invalid quantity format", 0, c.calls)
  }

  @Test fun `quantity120Accepted`() {
    val f = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray) =
        DatecsResponse(0x20, command,
          if (command == 56) "0\tBON-1\t".toByteArray() else "0\t".toByteArray(),
          byteArrayOf())
    }
    val sale = BlueCashSale(
      "op-120-1", "DY000600-OP01-0000001", 1, "1", 24,
      listOf(FiscalLine("Widget", 'B', "2.65", "120.000")),
      listOf(SalePayment("p", "CASH", "318.00"))
    )
    val result = BlueCashCommandProcessor(f, Card(), SignedMemoryJournal(Signer())).sale(sale)
    assertEquals("FISCALIZED", result["state"])
  }

  @Test fun `rejectsPriceTimesQuantityOver9999999_99`() {
    val c = Card()
    // 50000.00 × 200.000 = 10_000_000 which exceeds 9_999_999.99
    val bigSale = BlueCashSale(
      "op-big-1", "DY000600-OP01-0000001", 1, "1", 24,
      listOf(FiscalLine("Item", 'B', "50000.00", "200.000")),
      listOf(SalePayment("p", "CARD", "50000.00"))
    )
    val result = BlueCashCommandProcessor(Fiscal(), c, SignedMemoryJournal(Signer())).sale(bigSale)
    assertEquals("FAILED", result["state"])
    assertEquals("card must not be charged when total exceeds max", 0, c.calls)
  }

  @Test
  fun emptyCloseResponseIsNotSuccess() {
    val blankFiscal = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse =
        if (command == 56) DatecsResponse(0x20, command, "0\t\t".toByteArray(), byteArrayOf())
        else DatecsResponse(0x20, command, "0\t".toByteArray(), byteArrayOf())
    }
    val j = SignedMemoryJournal(Signer())
    val result = BlueCashCommandProcessor(blankFiscal, Card(), j).sale(sale())
    assertEquals("RECOVERY_REQUIRED", result["state"])
    assertEquals("FISCAL_REFERENCE_UNPARSEABLE", result["error_code"])
    assertNull(result["fiscal_reference"])
  }
}
