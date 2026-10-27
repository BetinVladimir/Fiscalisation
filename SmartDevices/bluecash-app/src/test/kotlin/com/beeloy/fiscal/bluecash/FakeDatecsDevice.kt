package com.beeloy.fiscal.bluecash

import java.io.InputStream
import java.io.OutputStream
import java.io.PipedInputStream
import java.io.PipedOutputStream

/**
 * FakeDatecsDevice for VG-03 SEQ tests.
 * Echoes valid Datecs response frames back to the protocol.
 */
class FakeDatecsDevice {
    private val toDevice = PipedOutputStream()
    private val fromDevice = PipedOutputStream()
    val deviceInput: InputStream = PipedInputStream(toDevice)
    val deviceOutput: OutputStream = PipedOutputStream(PipedInputStream().also {})

    companion object {
        /**
         * Build a minimal valid Datecs response frame for the given seq/cmd with data "0".
         * Used to satisfy the protocol's correlation check.
         */
        fun responseFrame(seq: Int, cmd: Int, data: ByteArray = "0".toByteArray()): ByteArray {
            return DatecsFrameCodec.encode(seq, cmd, data)
        }
    }
}
