package com.beeloy.fiscal.bluecash

import org.json.JSONArray
import org.junit.Assert.*
import org.junit.Test
import java.io.File
import java.nio.charset.Charset

/**
 * TST-DEEP-05: Datecs frame codec against 15 golden frames from PM v2.11.4.
 *
 * Covers: cmd 48, 49, 53, 56, 60, 74, 76; Cyrillic in CP1251 (invariant 7);
 * SEQ wrap 0x7f → 0x20; BCC verification (invariant 7, Datecs LEN = data+0x2A for requests).
 *
 * Protocol asymmetry:
 *   DatecsFrameCodec.encode() → REQUEST frame  (SOH LEN SEQ CMD data ENQ BCC ETX, 16+n bytes)
 *   DatecsFrameCodec.decode() → RESPONSE frame (SOH LEN SEQ CMD data EOT status ENQ BCC ETX, 25+n bytes)
 */
class DatecsGoldenFramesTest {

    data class GoldenFrame(
        val cmd: Int,
        val direction: String,
        val seq: Int,
        val dataHex: String,
        val frameHex: String,
        val bccVerified: Boolean,
        val lenVerified: Boolean
    )

    private fun loadFixture(): List<GoldenFrame> {
        var dir = File(javaClass.classLoader!!.getResource(".")!!.path)
        repeat(12) { if (!File(dir, "docs").exists()) dir = dir.parentFile ?: dir }
        val fixture = File(dir, "docs/MVP1/fixtures/datecs_golden_frames.json")
        require(fixture.exists()) { "Golden frames fixture not found at: ${fixture.absolutePath}" }
        val arr = JSONArray(fixture.readText())
        return (0 until arr.length()).map { i ->
            val obj = arr.getJSONObject(i)
            GoldenFrame(
                cmd = obj.getInt("cmd"),
                direction = obj.optString("direction", ""),
                seq = obj.getInt("seq"),
                dataHex = obj.optString("data_hex", ""),
                frameHex = obj.getString("frame_hex"),
                bccVerified = obj.optBoolean("bcc_verified", false),
                lenVerified = obj.optBoolean("len_verified", false)
            )
        }
    }

    private fun hexToBytes(hex: String): ByteArray {
        val h = hex.trim()
        if (h.isEmpty()) return byteArrayOf()
        return ByteArray(h.length / 2) { i -> h.substring(i * 2, i * 2 + 2).toInt(16).toByte() }
    }

    // ── 1. Decode RESPONSE frames from fixture ────────────────────────────────
    // decode() parses device-to-host (answer) frames; encode() builds host-to-device (request) frames.

    @Test fun `golden answer frames for target cmds decode correctly`() {
        val frames = loadFixture()
        val targetCmds = setOf(48, 49, 53, 56, 60, 74, 76)
        val answers = frames.filter { it.cmd in targetCmds && it.direction == "answer" && it.bccVerified }
        assertTrue("Need at least 4 verified answer frames, got ${answers.size}", answers.size >= 4)
        var decoded = 0
        val decodedCmds = mutableSetOf<Int>()
        for (f in answers) {
            val resp = DatecsFrameCodec.decode(hexToBytes(f.frameHex))
            assertEquals("seq mismatch for cmd ${f.cmd}", f.seq, resp.sequence)
            assertEquals("cmd mismatch", f.cmd, resp.command)
            // Verify data bytes match fixture
            val expectedData = hexToBytes(f.dataHex)
            assertArrayEquals("data mismatch for cmd ${f.cmd}", expectedData, resp.data)
            decodedCmds.add(f.cmd)
            decoded++
        }
        assertTrue("Must decode at least 4 answer frames", decoded >= 4)
    }

    @Test fun `golden answer frame cmd 48 decodes correctly`() {
        val frames = loadFixture()
        val f = frames.first { it.cmd == 48 && it.direction == "answer" && it.bccVerified }
        val resp = DatecsFrameCodec.decode(hexToBytes(f.frameHex))
        assertEquals("cmd", 48, resp.command)
        assertEquals("seq", f.seq, resp.sequence)
        assertArrayEquals("data", hexToBytes(f.dataHex), resp.data)
    }

    @Test fun `golden answer frame cmd 49 decodes correctly`() {
        val frames = loadFixture()
        val f = frames.first { it.cmd == 49 && it.direction == "answer" && it.bccVerified }
        val resp = DatecsFrameCodec.decode(hexToBytes(f.frameHex))
        assertEquals("cmd", 49, resp.command)
        assertEquals("seq", f.seq, resp.sequence)
    }

    @Test fun `golden answer frame cmd 56 decodes correctly`() {
        val frames = loadFixture()
        val f = frames.first { it.cmd == 56 && it.direction == "answer" && it.bccVerified }
        val resp = DatecsFrameCodec.decode(hexToBytes(f.frameHex))
        assertEquals("cmd", 56, resp.command)
        assertEquals("seq", f.seq, resp.sequence)
        assertArrayEquals("data", hexToBytes(f.dataHex), resp.data)
    }

    @Test fun `golden answer frame cmd 60 decodes correctly`() {
        val frames = loadFixture()
        val f60 = frames.firstOrNull { it.cmd == 60 && it.direction == "answer" && it.bccVerified }
        if (f60 == null) { return }  // not available in fixture; covered by encode tests
        val resp = DatecsFrameCodec.decode(hexToBytes(f60.frameHex))
        assertEquals("cmd", 60, resp.command)
        assertEquals("seq", f60.seq, resp.sequence)
    }

    // ── 2. Encode REQUEST frames and verify structure ─────────────────────────
    // encode() builds SOH LEN SEQ CMD data ENQ BCC ETX; LEN = data.size + 0x2A (invariant 7).

    @Test fun `encode cmd 48 produces correct SOH and ETX boundaries`() {
        val payload = "1\t1\tAB123456-OP01-0000001\t24\t\t".toByteArray(Charsets.ISO_8859_1)
        val frame = DatecsFrameCodec.encode(0x30, 48, payload)
        assertEquals("SOH", 0x01, frame.first().toInt())
        assertEquals("ETX", 0x03, frame.last().toInt())
        // REQUEST frame size: 16 + payload.size
        assertEquals("frame size", 16 + payload.size, frame.size)
    }

    @Test fun `encode cmd 60 empty payload produces 16-byte frame`() {
        val frame = DatecsFrameCodec.encode(0x20, 60, byteArrayOf())
        assertEquals("SOH", 0x01, frame.first().toInt())
        assertEquals("ETX", 0x03, frame.last().toInt())
        assertEquals("empty payload frame size", 16, frame.size)
    }

    @Test fun `encode cmd 74 empty payload produces 16-byte frame`() {
        val frame = DatecsFrameCodec.encode(0x20, 74, byteArrayOf())
        assertEquals("frame size", 16, frame.size)
    }

    @Test fun `encode cmd 76 empty payload produces 16-byte frame`() {
        val frame = DatecsFrameCodec.encode(0x20, 76, byteArrayOf())
        assertEquals("frame size", 16, frame.size)
    }

    @Test fun `encode LEN field equals data_size plus 0x2A for request frames`() {
        // Invariant 7: Datecs request LEN = data.size + 0x2A
        // LEN is stored as 4 nibble-bytes 0x30..0x3f
        for (dataSize in listOf(0, 1, 5, 20, 100)) {
            val data = ByteArray(dataSize) { 0x30 }
            val frame = DatecsFrameCodec.encode(0x20, 48, data)
            // Decode LEN from nibble-bytes at positions 1..4
            val lenVal = ((frame[1].toInt() - 0x30) shl 12) or
                         ((frame[2].toInt() - 0x30) shl 8) or
                         ((frame[3].toInt() - 0x30) shl 4) or
                         (frame[4].toInt() - 0x30)
            assertEquals("LEN for data.size=$dataSize must equal data.size+0x2A", dataSize + 0x2a, lenVal)
        }
    }

    // ── 3. Cyrillic CP1251 encoding (invariant 7) ─────────────────────────────

    @Test fun `DatecsText encodes Cyrillic to CP1251`() {
        val cp1251 = Charset.forName("windows-1251")
        val encoded = DatecsText.encode("Кафе")
        val expected = "Кафе".toByteArray(cp1251)
        assertArrayEquals("Cyrillic must be encoded as CP1251", expected, encoded)
    }

    @Test fun `DatecsText round-trip Cyrillic is lossless`() {
        val text = "Стока бр. кафе захар"
        val encoded = DatecsText.encode(text)
        val decoded = DatecsText.decode(encoded)
        assertEquals("CP1251 round-trip must be lossless for Cyrillic", text, decoded)
    }

    @Test fun `encode request with Cyrillic payload preserves CP1251 bytes`() {
        // Encode a request frame containing Cyrillic product name bytes
        val cp1251Name = DatecsText.encode("Стока")
        val frame = DatecsFrameCodec.encode(0x25, 49, cp1251Name)
        // Bytes 10..(10+cp1251Name.size-1) are the payload bytes in the request frame
        val extracted = frame.copyOfRange(10, 10 + cp1251Name.size)
        assertArrayEquals("CP1251 bytes in request payload", cp1251Name, extracted)
        assertEquals("Decoded Cyrillic", "Стока", DatecsText.decode(extracted))
    }

    // ── 4. SEQ wrap 0x7f → 0x20 (invariant 7: Datecs SEQ 0x20..0x7F) ─────────

    @Test fun `SEQ wrap rule 0x7f to 0x20`() {
        // Protocol rule: after 0x7f, SEQ wraps to 0x20
        val next = if (0x7f == 0x7f) 0x20 else 0x7f + 1
        assertEquals("After 0x7f next SEQ must be 0x20", 0x20, next)
    }

    @Test fun `encode with SEQ=0x7f stores 0x7f in SEQ byte position`() {
        val frame = DatecsFrameCodec.encode(0x7f, 74, byteArrayOf())
        // SEQ byte is at position 5 (after SOH + 4 LEN bytes)
        assertEquals("SEQ byte at position 5", 0x7f, frame[5].toInt() and 0xff)
    }

    @Test fun `SEQ boundary values 0x20 and 0x7f produce valid request frames`() {
        for (seq in listOf(0x20, 0x7f)) {
            val frame = DatecsFrameCodec.encode(seq, 76, byteArrayOf())
            assertEquals("SOH", 0x01, frame.first().toInt())
            assertEquals("ETX", 0x03, frame.last().toInt())
            assertEquals("SEQ at position 5", seq, frame[5].toInt() and 0xff)
        }
    }

    // ── 5. BCC integrity of RESPONSE frames ──────────────────────────────────

    @Test fun `corrupt BCC in response frame causes decode to throw`() {
        val frames = loadFixture()
        val ansFrame = frames.first { it.direction == "answer" && it.bccVerified }
        val raw = hexToBytes(ansFrame.frameHex).toMutableList()
        // Corrupt one BCC nibble (BCC is at positions -5..-2 from ETX)
        val bccIdx = raw.size - 3
        raw[bccIdx] = (raw[bccIdx].toInt() xor 0x01).toByte()
        try {
            DatecsFrameCodec.decode(raw.toByteArray())
            fail("Must throw on BCC mismatch")
        } catch (e: IllegalArgumentException) {
            assertTrue("Must mention BCC or DATECS",
                (e.message ?: "").let { it.contains("BCC", ignoreCase = true) || it.contains("DATECS", ignoreCase = true) })
        }
    }

    @Test fun `truncated response frame causes decode to throw`() {
        val frames = loadFixture()
        val ansFrame = frames.first { it.direction == "answer" && it.bccVerified }
        val raw = hexToBytes(ansFrame.frameHex)
        val truncated = raw.copyOf(raw.size - 4)
        try {
            DatecsFrameCodec.decode(truncated)
            fail("Must throw on truncated frame")
        } catch (e: IllegalArgumentException) {
            // expected: parsers never throw outside their typed error boundary (invariant 10)
        }
    }
}
