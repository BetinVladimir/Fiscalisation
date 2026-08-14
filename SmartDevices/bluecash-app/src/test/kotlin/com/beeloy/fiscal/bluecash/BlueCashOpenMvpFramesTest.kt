package com.beeloy.fiscal.bluecash

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.UUID

class BlueCashOpenMvpFramesTest {
    @Test
    fun `BFF1 aggregate round trip is digest bound and idempotent`() {
        val id = UUID.fromString("00000000-0000-4000-8000-000000000010")
        val payload = BlueCashCanonicalCbor.encode(mapOf("intent_id" to id.toString(), "action" to "PRINTER_TEST"))
        val frames = BlueCashOpenMvpFrames.encode(id, payload, 64)
        assertTrue(frames.size > 1)
        val assembler = BlueCashOpenMvpAssembler()
        var message: OpenMvpMessage? = null
        frames.forEach { message = assembler.accept(it) ?: message }
        assertEquals(id, message?.id)
        assertArrayEquals(payload, message?.payload)

        val bad = frames.last().clone().also { frame ->
            frame[frame.lastIndex] = (frame.last().toInt() xor 1).toByte()
        }
        val clean = BlueCashOpenMvpAssembler()
        frames.dropLast(1).forEach(clean::accept)
        assertThrows(IllegalArgumentException::class.java) { clean.accept(bad) }
    }
}
