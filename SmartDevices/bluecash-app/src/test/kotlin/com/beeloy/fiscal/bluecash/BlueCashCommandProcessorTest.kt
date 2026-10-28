package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

/** BUG-072: no mutual exclusion between MQTT/BLE/HTTP callers in BlueCashCommandProcessor. */
class BlueCashCommandProcessorTest {

  private fun key() = object : TransactionSigner {
    override val keyId = "test"
    override fun sign(hash: ByteArray) = hash
  }

  private fun journal() = SignedMemoryJournal(key())

  private fun cashSale(opId: String) = BlueCashSale(
    operationId = opId,
    unp = "DT000001-OP01-0000001",
    operator = 1,
    password = "0000",
    till = 1,
    lines = listOf(FiscalLine("Coffee", 'B', "13.67", "1.000", 0, "", 0, "")),
    payments = listOf(SalePayment("p1", "CASH", "13.67"))
  )

  private fun noCard() = object : DatecsPaymentPort {
    override fun reachable() = true
    override fun purchaseEur(amountMinor: Long, operationId: String) =
      mapOf("approved" to "true", "rrn" to "", "authorization_code" to "")
    override fun reverse(operationId: String) = mapOf("approved" to "true")
  }

  private fun simpleFiscal(onOpen: () -> Unit = {}) = object : DatecsFiscalPort {
    override fun reachable() = true
    override fun execute(command: Int, payload: ByteArray): DatecsResponse {
      if (command == 48) onOpen()
      val data = if (command == 56) "0\tBON-1\t".toByteArray() else "0\t".toByteArray()
      return DatecsResponse(0x20, command, data, byteArrayOf())
    }
  }

  // BUG-072.1: 8 concurrent callers with the same operationId → fiscal.execute(48) fires exactly once.
  @Test fun `concurrentDuplicateCommandsExecuteOnce`() {
    val openCount = AtomicInteger(0)
    val processor = BlueCashCommandProcessor(
      simpleFiscal { openCount.incrementAndGet() }, noCard(), journal()
    )
    val cmd = cashSale("OP-CONC-001")
    val n = 8
    val latch = CountDownLatch(n)
    val exec = Executors.newFixedThreadPool(n)
    repeat(n) {
      exec.submit {
        latch.countDown()
        latch.await()
        processor.sale(cmd)
      }
    }
    exec.shutdown()
    assertTrue("threads must finish in 5 s", exec.awaitTermination(5, TimeUnit.SECONDS))
    assertEquals(
      "fiscal.execute(48) must fire exactly once despite $n concurrent callers",
      1, openCount.get()
    )
  }

  // BUG-072.2: duplicate operationId returns the stored FISCALIZED result, not RECOVERY_REQUIRED.
  @Test fun `duplicateOperationReturnsStoredResult`() {
    val processor = BlueCashCommandProcessor(simpleFiscal(), noCard(), journal())
    val cmd = cashSale("OP-REPLAY-001")
    val first = processor.sale(cmd)
    val second = processor.sale(cmd)
    assertEquals("first call must FISCALIZE", "FISCALIZED", first["state"])
    assertEquals("replay must return same state", first["state"], second["state"])
    assertEquals(
      "replay must return same fiscal_reference",
      first["fiscal_reference"], second["fiscal_reference"]
    )
  }

  // BUG-072.3: two different operations are serialized — the second waits for the first to finish.
  @Test fun `differentOperationsSerialized`() {
    val openTimes = CopyOnWriteArrayList<Long>()
    val slowFiscal = object : DatecsFiscalPort {
      override fun reachable() = true
      override fun execute(command: Int, payload: ByteArray): DatecsResponse {
        if (command == 48) {
          openTimes.add(System.nanoTime())
          Thread.sleep(80)
        }
        val data = if (command == 56) "0\tBON-1\t".toByteArray() else "0\t".toByteArray()
        return DatecsResponse(0x20, command, data, byteArrayOf())
      }
    }
    val processor = BlueCashCommandProcessor(slowFiscal, noCard(), journal())
    val latch = CountDownLatch(2)
    val exec = Executors.newFixedThreadPool(2)
    exec.submit { latch.countDown(); latch.await(); processor.sale(cashSale("OP-SER-001")) }
    exec.submit { latch.countDown(); latch.await(); processor.sale(cashSale("OP-SER-002")) }
    exec.shutdown()
    assertTrue("threads must finish in 10 s", exec.awaitTermination(10, TimeUnit.SECONDS))
    assertEquals("both ops must reach fiscal.execute(48)", 2, openTimes.size)
    val sorted = openTimes.sorted()
    val gapMs = (sorted.last() - sorted.first()) / 1_000_000L
    assertTrue(
      "ops must be serialized: gap between execute(48) calls was ${gapMs}ms, expected >= 40ms",
      gapMs >= 40
    )
  }
}
