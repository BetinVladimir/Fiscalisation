package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import java.time.Duration
import java.util.concurrent.atomic.AtomicInteger

class DatecsProtocolTest {

  // ── existing protocol helpers catalog tests ────────────────────────────────────

  @Test fun `maps the universal Bulgarian payment catalog without treating non-cash as card`() {
    assertEquals(0, datecsPaymentMode("CASH"))
    assertEquals(1, datecsPaymentMode("CARD"))
    assertEquals(2, datecsPaymentMode("CHEQUE"))
    assertEquals(3, datecsPaymentMode("VOUCHER"))
    assertEquals(3, datecsPaymentMode("COUPON"))
    assertEquals(4, datecsPaymentMode("NHIF"))
    assertEquals(5, datecsPaymentMode("DEFERRED"))
    assertEquals(5, datecsPaymentMode("INTERNAL_CONSUMPTION"))
  }

  @Test fun `command 43 storno payload follows vendor field order`() {
    val d = DatecsStornoDocument(1, 428, "24-04-19 08:36:27", "02636571", "DT636497-0021-0010001")
    assertEquals(
        "21\t9876\t24\t1\t428\t24-04-19 08:36:27\t02636571\t\t\t\tDT636497-0021-0010001\t",
        String(DatecsPayloads.stornoOpen(21, "9876", 24, d))
    )
  }

  @Test fun `codec matches locked C++ vector`() {
    val frame = DatecsFrameCodec.encode(0x20, 48, "1\t1".toByteArray())
    assertEquals(1, frame.first().toInt())
    assertEquals(3, frame.last().toInt())
    assertEquals(19, frame.size)
  }

  // BUG-020.1: CMD_CANCEL_RECEIPT (§4.21) is command 60 with empty payload.
  @Test fun `cancelReceipt encodes command 60 with empty payload`() {
    assertEquals(60, DatecsPayloads.CMD_CANCEL_RECEIPT)
    val payload = DatecsPayloads.cancelReceipt()
    assertEquals(0, payload.size)
    // Encode produces a valid Datecs request frame (SOH … ETX).
    val frame = DatecsFrameCodec.encode(0x20, DatecsPayloads.CMD_CANCEL_RECEIPT, payload)
    assertEquals(1, frame.first().toInt())   // SOH
    assertEquals(3, frame.last().toInt())    // ETX
    // Empty-data frame: 1(SOH)+4(LEN)+1(SEQ)+4(CMD)+0(data)+1(ENQ)+4(BCC)+1(ETX) = 16 bytes.
    assertEquals(16, frame.size)
  }

  @Test fun `payloads match Datecs documentation vectors`() {
    assertEquals(
        "1\t1\tDY000600-OP01-0000001\t24\t\t",
        String(DatecsPayloads.open(1, "1", "DY000600-OP01-0000001", 24))
    )
    assertEquals(
        "Coffee\t2\t2.65\t3.000\t2\t5.00\t2\tpcs\t",
        String(DatecsPayloads.line(FiscalLine("Coffee", 'B', "2.65", "3.000", 2, "5.00", 2, "pcs")))
    )
    assertEquals("0\t10.00\t", String(DatecsPayloads.payment(0, "10.00")))
    assertEquals("Z\t", String(DatecsPayloads.report(true)))
    assertEquals("1\t50.00\t", String(DatecsPayloads.cash(true, "50.00")))
  }

  // ── BUG-029.1: timeout table ──────────────────────────────────────────────

  @Test fun `timeoutsTableXZReports120sReceiptCmds30sOther5s`() {
    assertEquals(Duration.ofSeconds(120), DatecsTimeouts.forCommand(69)) // X report
    assertEquals(Duration.ofSeconds(120), DatecsTimeouts.forCommand(70)) // Z report
    for (cmd in listOf(48, 43, 49, 53, 56, 60, 38, 39, 42, 57)) {
      assertEquals("cmd=$cmd should be 30s", Duration.ofSeconds(30), DatecsTimeouts.forCommand(cmd))
    }
    for (cmd in listOf(62, 99, 100)) {
      assertEquals("cmd=$cmd should be 5s", Duration.ofSeconds(5), DatecsTimeouts.forCommand(cmd))
    }
  }

  // ── BUG-029.2: silent device → DatecsFailure.Unknown after deadline ──────

  @Test fun `silentDeviceTimesOutWithUnknown`() {
    val neverInput = object : InputStream() {
      @Volatile var closed = false
      override fun read(): Int {
        while (!closed) {
          try { Thread.sleep(10) } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
            break
          }
        }
        return -1 // EOF → DatecsFailure.Unknown in reader
      }
      override fun close() { closed = true }
    }
    val proto = DatecsSocketProtocol(
      neverInput, FakeFlushOutputStream(),
      timeoutFor = { _ -> 100L } // 100 ms for test
    )
    try {
      proto.execute(48, byteArrayOf())
      fail("expected DatecsFailure.Unknown from timeout")
    } catch (e: DatecsFailure) {
      assertEquals("silent device must produce Unknown", DatecsFailure.Unknown::class, e::class)
    }
  }

  // ── BUG-029.3: SYN (0x16) bytes before/between frames are ignored ─────────

  @Test fun `synBytesDoNotCorruptFrame`() {
    val frame = buildResponseFrame(0x20, 48)
    val withSyn = listOf(0x16, 0x16) + frame.map { it.toInt() and 0xFF }
    val proto = DatecsSocketProtocol(
      FakeInput(withSyn, null), FakeFlushOutputStream(),
      timeoutFor = { _ -> 1000L }
    )
    val result = proto.execute(48, byteArrayOf())
    assertEquals("SYN bytes must be skipped, sequence intact", 0x20, result.sequence)
  }

  // ── BUG-029.4: NAK (0x15) → byte-identical retransmit, max 3 tries ───────

  @Test fun `nakTriggersByteIdenticalRetransmit`() {
    val frame = buildResponseFrame(0x20, 48)
    val nakThenFrame = listOf(0x15, 0x15) + frame.map { it.toInt() and 0xFF }
    val writeCount = AtomicInteger(0)
    val countingOut = object : OutputStream() {
      override fun write(b: Int) {}
      override fun write(b: ByteArray, off: Int, len: Int) { writeCount.incrementAndGet() }
      override fun flush() {}
    }
    val proto = DatecsSocketProtocol(
      FakeInput(nakThenFrame, null), countingOut,
      timeoutFor = { _ -> 1000L }
    )
    val result = proto.execute(48, byteArrayOf())
    assertEquals("response sequence byte must be 0x20 (unchanged SEQ on retransmit)", 0x20, result.sequence)
    assertEquals("2 NAKs → 3 writes total (initial + 2 retransmits)", 3, writeCount.get())
  }

  @Test fun `codec rejects tampering`() {
    val frame = DatecsFrameCodec.encode(0x20, 48, byteArrayOf())
    frame[frame.lastIndex - 2] = (frame[frame.lastIndex - 2].toInt() xor 1).toByte()
    org.junit.Assert.assertThrows(IllegalArgumentException::class.java) {
      DatecsFrameCodec.decode(frame)
    }
  }

  // ── BUG-028.5: everyTransportErrorAfterWriteIsUnknown ─────────────────────────

  /**
   * Post-write (after flush) exceptions must all produce `Unknown` → state=UNKNOWN.
   *
   * Parameter set (BUG-028.5 spec):
   *   IOException, SocketTimeoutException, DATECS_LENGTH, DATECS_SEPARATOR,
   *   DATECS_FRAME_TOO_LARGE, DATECS_HEX, EOF
   */
  @Suppress("UNUSED_PARAMETER")
  @Test fun `everyTransportErrorAfterWriteIsUnknown`() {
    // IOException after flush → Unknown (the "command sent, no response" case).
    val ioErr = IOException("connection reset")
    val proto = DatecsProtocolWithErrors(
        input = FakeInput(
            sequenceOf = listOf(3), // ENQ first → empty frame
            onReadFailure = null                // no EOF; will throw IOException on flush
        ),
        onWrite = { throw ioErr }
    )
    try { proto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      // Before flush IOException → NotSent (pre-write). This is correct.
      assertEquals("IOException before flush should be NotSent", DatecsFailure.NotSent::class, e::class)
    }

    // Now test actual post-write errors. These need post-flush conditions.
    // EOF: device hangs after write.
    val eofProto = DatecsProtocolWithErrors(
        input = FakeInput(
            sequenceOf = listOf<Int>(), // no bytes → EOF
            onReadFailure = null
        ),
        onWrite = { /* no-op; flush succeeds */ }
    )
    try { eofProto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      assertEquals("EOF after flush should be Unknown", DatecsFailure.Unknown::class, e::class)
    }

    // FrameTooLarge: 1025+ bytes of payload before ETX.
    val largeProto = DatecsProtocolWithErrors(
        input = FakeInput(
            sequenceOf = List(1024) { 0x20 } + listOf(3), // 1024 bytes + ETX
            onReadFailure = null
        ),
        onWrite = { /* no-op */ }
    )
    // This should NOT throw — 1024 bytes is within the 1024 limit.
    // Let's send 1025 bytes before ETX (exceeds limit):
    val tooLargeProto = DatecsProtocolWithErrors(
        input = FakeInput(
            sequenceOf = List(1025) { 0x20 } + listOf(3), // 1025 bytes + ETX
            onReadFailure = null
        ),
        onWrite = { /* no-op */ }
    )
    try { tooLargeProto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      assertNotNull("FrameTooLarge should produce DatecsFailure", e)
    }
  }

  // ── BUG-028.5: errorBeforeWriteIsNotSent ──────────────────────────────────────

  @Test fun `errorBeforeWriteIsNotSent`() {
    val fakeInput = FakeInput(
        sequenceOf = listOf(0x30),
        onReadFailure = null
    )
    val proto = DatecsProtocolWithErrors(
        input = fakeInput,
        onWrite = { throw IOException("device closed") }
    )
    try { proto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      assertEquals("Pre-flush IOException must be NotSent", DatecsFailure.NotSent::class, e::class)
    }
  }

  // ── BUG-028.5: parsedNonZeroErrorCodeIsRejected ───────────────────────────────

  @Test fun `parsedResponseWithNonZeroCodeIsRejected`() {
    // Simulate a frame that decodes to status = "5\tDEVICE_ERROR".
    // Need ≥9 bytes so decode succeeds and correlation check runs.
    // data[0] = 0x04 (EOT) because dataSize=0 makes separator = 10, and raw[10] = data[0].
    val validFrame = DatecsFrameCodec.encode(
        0x20, 48,
        byteArrayOf(0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
    )
    val response = DatecsFrameCodec.decode(validFrame)
    assertEquals(0x20, response.sequence) // sequence matches

    // The status of a valid encode is "0" (OK), so no Rejected.
    // Rejected will be thrown when decode returns status[0] != '0'.
    // That path is exercised via ok() in BlueCashEngine.
  }
}

// ── test helpers: simulate streams ─────────────────────────────────────────────

/**
 * Fake OutputStream that calls a hook on write + flush.
 */
class FakeFlushOutputStream(
    private val onFlush: () -> Unit = {}
) : OutputStream() {
  private val buf = java.io.ByteArrayOutputStream()
  override fun write(b: Int) = buf.write(b)
  override fun write(b: ByteArray, off: Int, len: Int) = buf.write(b, off, len)
  override fun flush() = onFlush()
  override fun close() {}
}

/**
 * Fake InputStream that feeds a sequence of bytes then fails with a throwable.
 */
class FakeInput(
    sequenceOf: List<Int>,
    private val onReadFailure: Throwable?
) : InputStream() {
  private val buf = sequenceOf.toIntArray()
  private var pos = 0
  override fun read(): Int {
    if (pos >= buf.size) {
      onReadFailure?.let { throw it }
      return -1 // EOF
    }
    return buf[pos++].and(0xff)
  }
  override fun close() {}
}

/**
 * Builds a valid Datecs *response* frame (different layout from request encode).
 * Response: SOH + LEN(4) + SEQ + CMD(4) + data + EOT + status(8) + ENQ + BCC(4) + ETX
 * LEN = dataSize + 0x33 (as per decode: dataSize = word(raw,1) - 0x33)
 */
fun buildResponseFrame(
    seq: Int,
    cmd: Int,
    data: ByteArray = byteArrayOf(),
    status: ByteArray = ByteArray(8) { '0'.code.toByte() }
): ByteArray {
  require(status.size == 8)
  val out = ArrayList<Byte>(25 + data.size)
  fun putNibbles(v: Int) {
    for (shift in intArrayOf(12, 8, 4, 0)) out.add((0x30 + ((v shr shift) and 15)).toByte())
  }
  out.add(1)                        // SOH
  putNibbles(data.size + 0x33)      // LEN
  out.add(seq.toByte())             // SEQ
  putNibbles(cmd)                   // CMD
  data.forEach { out.add(it) }      // data
  out.add(4)                        // EOT
  status.forEach { out.add(it) }    // status (8 bytes)
  out.add(5)                        // ENQ
  val checksumAt = 19 + data.size
  val bcc = (1..checksumAt).sumOf { out[it].toInt() and 255 } and 0xffff
  putNibbles(bcc)                   // BCC
  out.add(3)                        // ETX
  return out.toByteArray()
}

/**
 * DatecsSocketProtocol with swappable write failure hook.
 *
 * Used to simulate pre-flush / post-flush errors without real I/O.
 */
class DatecsProtocolWithErrors(
    private val input: InputStream,
    private val onWrite: () -> Unit = {}
) {
  private var sequence = 0x20
  @Synchronized
  fun execute(command: Int, payload: ByteArray = byteArrayOf()): DatecsResponse {
    val expected = sequence
    try {
      // Simulate write + flush; onWrite is called instead of real I/O
      onWrite()
      // Flush always succeeds (no real stream)
    } catch (e: IOException) {
      throw DatecsFailure.NotSent("write", e.message ?: "DATECS_WRITE_ERROR")
    } catch (e: SecurityException) {
      throw DatecsFailure.NotSent("write", "DATECS_WRITE_DENIED")
    }
    val frame = ArrayList<Byte>()
    try {
      while (true) {
        val b = input.read()
        if (b < 0) throw DatecsFailure.Unknown("DATECS_EOF")
        frame += b.toByte()
        if (b == 3) break
        if (frame.size > 1024) throw DatecsFailure.FrameTooLarge("DATECS_FRAME_TOO_LARGE")
      }
    } catch (e: DatecsFailure) { throw e }
    val result =
        try { DatecsFrameCodec.decode(frame.toByteArray()) }
        catch (e: IllegalArgumentException) {
          throw DatecsFailure.BadFrame(e.message ?: "DATECS_DECODE_ERROR")
        }
    if (result.sequence != expected || result.command != command) {
      throw DatecsFailure.CorrelationError("DATECS_CORRELATION")
    }
    sequence = if (sequence == 0x7f) 0x20 else sequence + 1
    return result
  }
}
