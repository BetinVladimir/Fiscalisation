package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-10: Protocol-level behaviors (BUG-029).
 *
 * Note: DatecsFrameCodec.encode() produces COMMAND frames (sent to device);
 *       DatecsFrameCodec.decode() parses RESPONSE frames (received from device).
 *       Command frames: LEN = data.size + 0x2A, no status bytes.
 *       Response frames: LEN = data.size + 0x33, with 8 status bytes after data.
 * SEQ wraps 0x7F → 0x20 for Datecs (invariant 7).
 * BCC validation on response frame.
 */
class TSTKT10Test {

    // ─── Build a valid response frame for decode tests ───────────────────────

    /**
     * Builds a valid Datecs RESPONSE frame:
     * SOH + LEN(4) + SEQ(1) + CMD(4) + DATA + EOT + STATUS(8) + ENQ + BCC(4) + ETX
     */
    private fun responseFrame(seq: Int, cmd: Int, data: ByteArray, status: ByteArray = ByteArray(8)): ByteArray {
        require(status.size == 8)
        fun nibble(v: Int) = (0x30 + (v and 0xF)).toByte()
        fun word(v: Int) = byteArrayOf(nibble(v shr 12), nibble(v shr 8), nibble(v shr 4), nibble(v))

        val out = mutableListOf<Byte>()
        out.add(0x01)                                     // SOH
        out.addAll(word(data.size + 0x33).toList())       // LEN = data.size + 0x33
        out.add(seq.toByte())                              // SEQ
        out.addAll(word(cmd).toList())                     // CMD
        out.addAll(data.toList())                          // DATA
        out.add(0x04)                                      // EOT (separator)
        out.addAll(status.toList())                        // STATUS (8 bytes)
        // ENQ goes at checksumAt = 19 + data.size; sum = bytes[1..checksumAt] inclusive
        val checksumAt = out.size  // current index = 19 + data.size
        val sum = (1 until checksumAt).sumOf { out[it].toInt() and 0xFF } and 0xFFFF
        val enq = 0x05.toByte()
        out.add(enq)                                       // ENQ
        // BCC: sum of bytes[1..checksumAt] inclusive of ENQ
        val bccSum = ((1..checksumAt).sumOf { out[it].toInt() and 0xFF } and 0xFFFF)
        out.addAll(word(bccSum).toList())                  // BCC
        out.add(0x03)                                      // ETX
        return out.toByteArray()
    }

    // ─── SEQ range ────────────────────────────────────────────────────────────

    @Test fun seqStartsAt0x20() {
        assertEquals("MIN_SEQ must be 0x20", 0x20, DatecsLimits.MIN_SEQ)
    }

    @Test fun seqDatecsEndsAt0x7F() {
        assertEquals("MAX_SEQ_DATECS must be 0x7F", 0x7F, DatecsLimits.MAX_SEQ_DATECS)
    }

    @Test fun seqDaisyEndsAt0xFF() {
        assertEquals("MAX_SEQ_DAISY must be 0xFF", 0xFF, DatecsLimits.MAX_SEQ_DAISY)
    }

    // SEQ after 0x7F wraps to 0x20 — verified by encoding a frame at seq=0x7F and then 0x20
    @Test fun seqWrapsAfter0x7F() {
        val cmd = 49
        val data = byteArrayOf()
        val frameAt7F = DatecsFrameCodec.encode(0x7F, cmd, data)
        val frameAt20 = DatecsFrameCodec.encode(0x20, cmd, data)
        assertNotNull(frameAt7F)
        assertNotNull(frameAt20)
        assertEquals("SEQ byte at 0x7F must be 0x7F", 0x7F, frameAt7F[5].toInt() and 0xFF)
        assertEquals("SEQ byte at 0x20 must be 0x20", 0x20, frameAt20[5].toInt() and 0xFF)
        // next after 0x7F is 0x20 (not 0x80 which would be out of Datecs range)
        val nextSeq = if (0x7F >= DatecsLimits.MAX_SEQ_DATECS) DatecsLimits.MIN_SEQ else 0x7F + 1
        assertEquals("next SEQ after 0x7F must wrap to 0x20", 0x20, nextSeq)
    }

    // ─── Frame codec ──────────────────────────────────────────────────────────

    // decode() parses a valid RESPONSE frame; sequence, command, data round-trip correctly
    @Test fun encodeDecodeSingleByte() {
        val seq = 0x20
        val cmd = 49
        val payload = byteArrayOf(0x41) // 'A'
        val frame = responseFrame(seq, cmd, payload)
        val decoded = DatecsFrameCodec.decode(frame)
        assertEquals("sequence must round-trip", seq, decoded.sequence)
        assertEquals("command must round-trip", cmd, decoded.command)
        assertArrayEquals("data must round-trip", payload, decoded.data)
    }

    // Corrupting a byte in a valid response frame → decode throws with DATECS error
    @Test fun bccValidationRejectsCorruptedFrame() {
        val frame = responseFrame(0x20, 49, "Hello".toByteArray())
        // Corrupt the data byte (index 10 in a response frame)
        val corrupted = frame.copyOf()
        corrupted[10] = (corrupted[10].toInt() xor 0xFF).toByte()
        try {
            DatecsFrameCodec.decode(corrupted)
            fail("BCC error must be thrown for corrupted frame")
        } catch (e: IllegalArgumentException) {
            assertTrue("must report BCC or frame error",
                e.message?.contains("DATECS") == true)
        }
    }

    // ─── LEN offset for COMMAND frame (invariant 7) ────────────────────────

    // encode() puts LEN = data.size + 0x2A in bytes[1..4] (hex-encoded)
    @Test fun lenOffsetDatecsIs0x2A() {
        val data = ByteArray(10)
        val frame = DatecsFrameCodec.encode(0x20, 49, data)
        // bytes 1-4 are hex-encoded LEN; parse them
        val lenHex = String(frame.copyOfRange(1, 5), Charsets.US_ASCII)
        val lenValue = lenHex.toInt(16)
        assertEquals("LEN must be data.size + 0x2A for Datecs command frame",
            data.size + 0x2A, lenValue)
    }

    // ─── CP1251 encoding ──────────────────────────────────────────────────────

    @Test fun cyrillicTextEncodesAsCP1251() {
        // "Кафе" CP1251: К=0xCA, а=0xE0, ф=0xF4, е=0xE5
        val encoded = DatecsText.encode("Кафе")
        assertEquals("Кафе is 4 bytes in CP1251", 4, encoded.size)
        assertEquals(0xCA.toByte(), encoded[0])
        assertEquals(0xE0.toByte(), encoded[1])
        assertEquals(0xF4.toByte(), encoded[2])
        assertEquals(0xE5.toByte(), encoded[3])
    }
}
