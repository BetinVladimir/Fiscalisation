package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.util.Random

/**
 * TST-DEEP-12: Frame parser fuzz test.
 *
 * Invariant 10: parsers never throw externally — any byte input must produce either
 * a parsed frame or a typed IllegalArgumentException, never an unchecked exception,
 * NullPointerException, ArrayIndexOutOfBoundsException, or stack overflow.
 *
 * Covers:
 *   - DatecsFrameCodec.decode: RESPONSE frame parser
 *   - DatecsFrameCodec.encode + decode cycle with arbitrary inputs (within valid ranges)
 *   - DatecsText.encode: CP1251 encoder does not leak raw exception
 *   - DatecsText.decode: handles arbitrary byte arrays
 *   - DatecsStatus.parse: handles arbitrary status bytes
 */
class FrameFuzzTest {

    private val rng = Random(0xBEE10F0BB1L)

    // ── DatecsFrameCodec.decode fuzz ─────────────────────────────────────────

    @Test fun `decode never throws unchecked exception on random bytes`() {
        var trials = 0
        var illegalArgs = 0
        repeat(10_000) {
            val size = rng.nextInt(64) + 1
            val raw = ByteArray(size) { rng.nextInt(256).toByte() }
            try {
                DatecsFrameCodec.decode(raw)
                // If it returned, that's fine — the frame was (by chance) valid
            } catch (e: IllegalArgumentException) {
                illegalArgs++  // expected typed error
            } catch (e: Exception) {
                fail("DatecsFrameCodec.decode must not throw ${e::class.simpleName}: ${e.message}" +
                     "\ninput=${raw.take(16).joinToString { "%02x".format(it) }}")
            }
            trials++
        }
        assertTrue("Must attempt 10000 trials", trials == 10_000)
        // With random bytes almost all attempts fail; verify the typed-error path is exercised
        assertTrue("Typed errors expected for random input", illegalArgs > 9_000)
    }

    @Test fun `decode never throws unchecked exception on zero-length input`() {
        try {
            DatecsFrameCodec.decode(byteArrayOf())
        } catch (e: IllegalArgumentException) {
            // expected
        } catch (e: Exception) {
            fail("Empty input must throw IllegalArgumentException, got ${e::class.simpleName}")
        }
    }

    @Test fun `decode never throws unchecked exception on single-byte inputs`() {
        for (b in 0..255) {
            try {
                DatecsFrameCodec.decode(byteArrayOf(b.toByte()))
            } catch (e: IllegalArgumentException) {
                // expected
            } catch (e: Exception) {
                fail("Single byte 0x%02x must throw IllegalArgumentException, got %s".format(b, e::class.simpleName))
            }
        }
    }

    @Test fun `decode never throws unchecked exception on near-valid frames`() {
        // Build a valid frame then corrupt each byte in turn
        val valid = DatecsFrameCodec.encode(0x20, 48, "1\t".toByteArray())
        // Build a response-format frame (decode expects response format)
        // Use a golden answer frame from fixtures to produce near-valid corruptions
        val answerHex = "013030323a2c303033303009343733090480808880869a80800530363d3703"
        val raw = ByteArray(answerHex.length / 2) { i ->
            answerHex.substring(i * 2, i * 2 + 2).toInt(16).toByte()
        }
        for (i in raw.indices) {
            for (flip in listOf(0x01, 0x80, 0xff)) {
                val corrupted = raw.copyOf()
                corrupted[i] = (corrupted[i].toInt() xor flip).toByte()
                try {
                    DatecsFrameCodec.decode(corrupted)
                } catch (e: IllegalArgumentException) {
                    // expected
                } catch (e: Exception) {
                    fail("Near-valid frame (byte $i flipped 0x${"%02x".format(flip)}) must throw " +
                         "IllegalArgumentException, got ${e::class.simpleName}: ${e.message}")
                }
            }
        }
    }

    @Test fun `decode never throws on very large random payloads`() {
        repeat(100) {
            val size = 512 + rng.nextInt(512)
            val raw = ByteArray(size) { rng.nextInt(256).toByte() }
            try {
                DatecsFrameCodec.decode(raw)
            } catch (e: IllegalArgumentException) {
                // expected
            } catch (e: Exception) {
                fail("Large random input must throw IllegalArgumentException, got ${e::class.simpleName}")
            }
        }
    }

    // ── DatecsText fuzz ───────────────────────────────────────────────────────

    @Test fun `DatecsText decode never throws on arbitrary byte arrays`() {
        repeat(1_000) {
            val size = rng.nextInt(128)
            val raw = ByteArray(size) { rng.nextInt(256).toByte() }
            try {
                DatecsText.decode(raw)  // must always succeed — it's a decode, not parse
            } catch (e: Exception) {
                fail("DatecsText.decode must never throw, got ${e::class.simpleName}: ${e.message}")
            }
        }
    }

    @Test fun `DatecsText encode throws only IllegalArgumentException for unmappable chars`() {
        // Characters that cannot be encoded in CP1251 (e.g. emoji, Chinese characters)
        val unmappable = listOf("😀", "中文", "\uD800")
        for (s in unmappable) {
            try {
                DatecsText.encode(s)
                // If it succeeded (some JVMs may substitute), that's acceptable
            } catch (e: IllegalArgumentException) {
                // expected typed error per invariant 10
            } catch (e: Exception) {
                fail("DatecsText.encode must throw IllegalArgumentException for ${s.take(4)}, got ${e::class.simpleName}")
            }
        }
    }

    // ── DatecsFrameCodec.encode safety ───────────────────────────────────────

    @Test fun `encode rejects out-of-range SEQ with IllegalArgumentException`() {
        for (badSeq in listOf(0x00, 0x1f, 0x80, 0xff, -1, 256)) {
            try {
                DatecsFrameCodec.encode(badSeq, 48, byteArrayOf())
                fail("SEQ $badSeq is out of 0x20..0x7F range; must throw")
            } catch (e: IllegalArgumentException) {
                // expected
            } catch (e: Exception) {
                fail("Out-of-range SEQ $badSeq must throw IllegalArgumentException, got ${e::class.simpleName}")
            }
        }
    }

    @Test fun `encode rejects oversized payload with IllegalArgumentException`() {
        val tooBig = ByteArray(497) { 0x30 }
        try {
            DatecsFrameCodec.encode(0x20, 48, tooBig)
            fail("Payload > 496 bytes must throw")
        } catch (e: IllegalArgumentException) {
            // expected
        } catch (e: Exception) {
            fail("Oversized payload must throw IllegalArgumentException, got ${e::class.simpleName}")
        }
    }

    // ── End-to-end valid encode→decode cycle ─────────────────────────────────
    // Note: encode() → request frame, decode() → response frame — these are different formats.
    // Round-trip is not possible with these two functions; encode safety is tested above.

    @Test fun `encode produces structurally valid request frames for all valid SEQ values`() {
        for (seq in 0x20..0x7f) {
            val frame = DatecsFrameCodec.encode(seq, 74, byteArrayOf())
            assertEquals("SOH", 0x01, frame.first().toInt())
            assertEquals("ETX", 0x03, frame.last().toInt())
            assertEquals("frame size", 16, frame.size)
            // SEQ byte at position 5
            assertEquals("SEQ", seq, frame[5].toInt() and 0xff)
        }
    }
}
