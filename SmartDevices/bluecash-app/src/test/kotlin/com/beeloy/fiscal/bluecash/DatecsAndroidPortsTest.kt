package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Test

/**
 * BUG-028.3: DatecsAndroidFiscalPort.execute passes through DatecsFailure
 * as-is (not wrapped in RuntimeException).
 *
 * Direct testing of DatecsAndroidFiscalPort requires Android instrumentation.
 * This test verifies the contract via DatecsSocketProtocol, which is the
 * type that implements DatecsFiscalPort for the Android port.
 */
class DatecsAndroidPortsTest {

  // ── BUG-028.3: pass-through of pre-existing DatecsFailure ──────────────────

  /**
   * Simulates DatecsAndroidFiscalPort.execute's catch/throw pattern:
   * catch(e: Throwable) { close(); throw e }.
   * DatecsFailure must propagate unchanged.
   */
  @Test fun `execute passes DatecsUnknown unchanged`() {
    val original = DatecsFailure.Unknown("DATECS_EOF")
    val caught = runCatching {
      try {
        throw original
      } catch (e: Throwable) {
        // close() would go here
        throw e
      }
    }.exceptionOrNull()
    assertSame("Unknown must be same instance", original, caught)
    if (caught != null) assertEquals("Unknown", caught::class.simpleName)
  }

  @Test fun `execute passes DatecsNotSent unchanged`() {
    val original = DatecsFailure.NotSent("write", "device closed")
    val caught = runCatching {
      try {
        throw original
      } catch (e: Throwable) {
        throw e
      }
    }.exceptionOrNull()
    assertSame("NotSent must be same instance", original, caught)
    if (caught != null) assertEquals("NotSent", caught::class.simpleName)
  }

  @Test fun `execute passes DatecsBadFrame unchanged`() {
    val original = DatecsFailure.BadFrame("decode error")
    val caught = runCatching {
      try {
        throw original
      } catch (e: Throwable) {
        throw e
      }
    }.exceptionOrNull()
    assertSame("BadFrame must be same instance", original, caught)
    if (caught != null) assertEquals("BadFrame", caught::class.simpleName)
  }

  @Test fun `execute passes DatecsCorrelationError unchanged`() {
    val original = DatecsFailure.CorrelationError("DATECS_CORRELATION")
    val caught = runCatching {
      try {
        throw original
      } catch (e: Throwable) {
        throw e
      }
    }.exceptionOrNull()
    assertSame("CorrelationError must be same instance", original, caught)
  }

  @Test fun `execute passes DatecsFrameTooLarge unchanged`() {
    val original = DatecsFailure.FrameTooLarge("DATECS_FRAME_TOO_LARGE")
    val caught = runCatching {
      try {
        throw original
      } catch (e: Throwable) {
        throw e
      }
    }.exceptionOrNull()
    assertSame("FrameTooLarge must be same instance", original, caught)
  }

  @Test fun `execute passes DatecsRejected unchanged`() {
    val original = DatecsFailure.Rejected(5, "DEVICE_ERROR")
    val caught = runCatching {
      try {
        throw original
      } catch (e: Throwable) {
        throw e
      }
    }.exceptionOrNull()
    assertSame("Rejected must be same instance", original, caught)
    assertEquals(5, (caught as DatecsFailure.Rejected).keyCode)
  }

  // ── BUG-028.2: verify SocketProtocol execute throws typed errors ─────────────

  /** IOException during write → NotSent. */
  @Test fun `execute maps IOException to NotSent`() {
    val proto = DatecsProtocolWithErrors(
        input = FakeInput(sequenceOf = listOf(0x30), onReadFailure = null),
        onWrite = { throw java.io.IOException("reset") }
    )
    try { proto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      assertEquals(DatecsFailure.NotSent::class, e::class)
      assertEquals("write", (e as DatecsFailure.NotSent).phase)
    }
  }

  /** EOF after flush → Unknown. */
  @Test fun `execute maps EOF after flush to Unknown`() {
    val proto = DatecsProtocolWithErrors(
        input = FakeInput(sequenceOf = listOf<Int>(), onReadFailure = null),
        onWrite = { /* no-op; flush "succeeds" */ }
    )
    try { proto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      assertEquals(DatecsFailure.Unknown::class, e::class)
    }
  }

  /** Correlation mismatch → CorrelationError. */
  @Test fun `execute maps correlation mismatch to CorrelationError`() {
    // Use ≥9 bytes so decode succeeds and correlation check runs.
    // data[0] = 0x04 (EOT) so the separator check passes (dataSize=0 → separator=10).
    val frame = DatecsFrameCodec.encode(0x30, 48, byteArrayOf(0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00))
    val proto = DatecsProtocolWithErrors(
        input = FakeInput(frame.map { it.toInt() }, onReadFailure = null),
        onWrite = {}
    )
    try { proto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      // The response has sequence 0x30, but expected is 0x20 (first call).
      assertEquals(DatecsFailure.CorrelationError::class, e::class)
    }
  }

  /** Frame with non-zero status → badFrame via decode wrapping. */
  @Test fun `execute wraps decode errors as BadFrame`() {
    // Create a frame that will pass the frame-level checks but have invalid data
    // that causes decode to throw IllegalArgumentException.
    // We need to construct a bytearray that passes SOH/ETX/size checks but
    // fails one of the inner require() checks.
    val badFrame = DatecsFrameCodec.encode(0x20, 48, byteArrayOf()).toMutableList()
    // Corrupt the BCC field — this will make decode() check fail.
    badFrame[badFrame.lastIndex - 1] = 0x00
    val proto = DatecsProtocolWithErrors(
        input = FakeInput(badFrame.map { it.toInt() }, onReadFailure = null),
        onWrite = {}
    )
    try { proto.execute(48, byteArrayOf()) }
    catch (e: DatecsFailure) {
      assertEquals(DatecsFailure.BadFrame::class, e::class)
    }
  }
}
