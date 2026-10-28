package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

/**
 * BUG-028 tests: error classification uses typed DatecsFailure hierarchy, not text matching.
 *
 * Invariant: only true communication loss → UNKNOWN; all protocol errors → FAILED.
 */
class BlueCashEngineUnknownTest {

  // ── helpers ──────────────────────────────────────────────────────────────────

  private fun makeProcessor(
      fiscal: MockFiscalPort,
      card: MockPaymentPort = MockPaymentPort()
  ): BlueCashCommandProcessor {
    val signer = object : TransactionSigner {
      override val keyId: String = "test-key"
      override fun sign(hash: ByteArray): ByteArray = hash
    }
    val journal = SignedMemoryJournal(signer, { java.time.Instant.now() })
    return BlueCashCommandProcessor(fiscal, card, journal)
  }

  private val testSale = BlueCashSale(
      operationId = "op-001",
      unp = "DY000600-A001-0000001",
      operator = 1,
      password = "0000",
      till = 1,
      lines = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
      payments = listOf(SalePayment("p-card-1", "CASH", "13.67"))
  )

  private val testReversal = BlueCashReversal(
      operationId = "rev-001",
      originalOperationId = "op-001",
      operator = 1,
      password = "0000",
      till = 1,
      originalDocument = DatecsStornoDocument(
          reason = 0,
          documentNumber = 428,
          documentDateTime = "24-04-19 08:36:27",
          fiscalMemoryNumber = "02636571",
          originalUnp = "DT636497-0021-0010001"
      ),
      lines = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
      payments = listOf(SalePayment("p-card-1", "CARD", "13.67"))
  )

  // ── BUG-028.4: BAD_FRAME → FAILED (not UNKNOWN) ────────────────────────────

  @Test fun `badFrame must NOT be UNKNOWN`() {
    val fiscal = MockFiscalPort(DatecsFailure.BadFrame("BAD FRAME"))
    val processor = makeProcessor(fiscal)
    val result = processor.sale(testSale)
    assertEquals("FAILED", result["state"])
    assertNotEquals("UNKNOWN", result["state"])
  }

  // ── BUG-028.4: BCC_ERROR → FAILED ──────────────────────────────────────────

  @Test fun `bccError must NOT be UNKNOWN`() {
    val fiscal = MockFiscalPort(DatecsFailure.BccError("BCC mismatch"))
    val processor = makeProcessor(fiscal)
    val result = processor.sale(testSale)
    assertEquals("FAILED", result["state"])
    assertNotEquals("UNKNOWN", result["state"])
  }

  // ── BUG-028.4: CORRELATION_ERROR → FAILED ──────────────────────────────────

  @Test fun `correlationError must NOT be UNKNOWN`() {
    val fiscal = MockFiscalPort(DatecsFailure.CorrelationError("seq mismatch"))
    val processor = makeProcessor(fiscal)
    val result = processor.sale(testSale)
    assertEquals("FAILED", result["state"])
    assertNotEquals("UNKNOWN", result["state"])
  }

  // ── BUG-028.4: Unknown → UNKNOWN ───────────────────────────────────────────

  @Test fun `unknown IS still UNKNOWN`() {
    val fiscal = MockFiscalPort(DatecsFailure.Unknown("no response"))
    val processor = makeProcessor(fiscal)
    val result = processor.sale(testSale)
    assertEquals("UNKNOWN", result["state"])
  }

  // ── BUG-028.4: Rejected → FAILED ───────────────────────────────────────────

  @Test fun `rejected must NOT be UNKNOWN`() {
    val fiscal = MockFiscalPort(DatecsFailure.Rejected(1, "DEVICE_ERROR"))
    val processor = makeProcessor(fiscal)
    val result = processor.sale(testSale)
    assertEquals("FAILED", result["state"])
  }

  // ── BUG-028.4: NotSent → FAILED ────────────────────────────────────────────

  @Test fun `notSent must NOT be UNKNOWN`() {
    val fiscal = MockFiscalPort(DatecsFailure.NotSent("write", "IO error"))
    val processor = makeProcessor(fiscal)
    val result = processor.sale(testSale)
    assertEquals("FAILED", result["state"])
  }

  // ── BUG-028.4: reverse path also uses typed classification ─────────────────

  @Test fun `reversePath_badFrame_isNotUnknown`() {
    val fiscal = MockFiscalPort(DatecsFailure.BadFrame("bad"))
    val processor = makeProcessor(fiscal)
    val result = processor.reverse(testReversal)
    assertEquals("FAILED", result["state"])
    assertNotEquals("UNKNOWN", result["state"])
  }

  // ── BUG-028.4: printerTest uses typed classification ───────────────────────

  @Test fun `printerTest_badFrame_isNotUnknown`() {
    val fiscal = MockFiscalPort(DatecsFailure.BadFrame("bad"))
    val signer = object : TransactionSigner {
      override val keyId: String = "k"
      override fun sign(hash: ByteArray): ByteArray = hash
    }
    val journal = SignedMemoryJournal(signer, { java.time.Instant.now() })
    val processor = BlueCashCommandProcessor(fiscal, MockPaymentPort(), journal)
    val result = processor.printerTest("pt-001")
    assertEquals("FAILED", result["state"])
    assertNotEquals("UNKNOWN", result["state"])
  }

  // ── BUG-028.4: administrative path ─────────────────────────────────────────

  @Test fun `adminPath_badFrame_isNotUnknown`() {
    val fiscal = MockFiscalPort(DatecsFailure.BadFrame("bad"))
    val result = makeProcessor(fiscal).administrative("admin-001", 69, byteArrayOf())
    assertEquals("FAILED", result["state"])
  }

  // ── BUG-028.4: unknown IS UNKNOWN for administrative ───────────────────────

  @Test fun `adminPath_unknown_isUnknown`() {
    val fiscal = MockFiscalPort(DatecsFailure.Unknown("no response"))
    val result = makeProcessor(fiscal).administrative("admin-002", 69, byteArrayOf())
    assertEquals("UNKNOWN", result["state"])
  }
}

// ── test doubles (same package → no import needed) ─────────────────────────────

/** Mock fiscal port that throws a controlled exception on execute. */
class MockFiscalPort(
    private val throwable: Throwable? = null
) : DatecsFiscalPort {
  override fun execute(command: Int, payload: ByteArray): DatecsResponse {
    throwable?.let { throw it }
    return FiscalMockResponse
  }
  override fun reachable(): Boolean = true
}

/** A mock success response. */
val FiscalMockResponse = DatecsResponse(
    sequence = 0x20,
    command = 56,
    data = "0\t000001\t20240101\t123456".toByteArray(Charsets.UTF_8),
    status = byteArrayOf('0'.toByte())
)

/** Mock payment port with no-op approve/reject. */
class MockPaymentPort : DatecsPaymentPort {
  override fun purchaseEur(amountMinor: Long, operationId: String) =
      mapOf("approved" to "true", "rrn" to "RRN123", "authorization_code" to "AC456")
  override fun reverse(operationId: String) =
      mapOf("approved" to "true", "rrn" to "RRN999", "authorization_code" to "AC000")
  override fun reachable(): Boolean = true
}
