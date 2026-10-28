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

/**
 * VG-04.5: Stateful FU emulator for DatecsRecovery tests.
 *
 * Simulates Datecs FU state table for cmd 60/76/255 responses.
 * Implements DatecsFiscalPort so it can be injected into DatecsRecovery.
 */
class StatefulFakeDatecsFiscalPort(
    var isOpen: Int = 0,
    var receiptNumber: Int = 0,
    var currentUnp: String = "",
    var lastFiscalDoc: Int = 0,
    var fmNumber: String = "12345678",
    var currencyMode: Int = 3
) : DatecsFiscalPort {

    val executedCommands = mutableListOf<Pair<Int, String>>()

    override fun reachable() = true

    override fun execute(command: Int, payload: ByteArray): DatecsResponse {
        val payloadStr = DatecsText.decode(payload)
        executedCommands += command to payloadStr

        val responseData: String = when (command) {
            // cmd 76: read receipt status → IsOpen\tNumber\tItems\tAmount\tPayed
            76 -> "0\t$isOpen\t$receiptNumber\t0\t0.00\t0.00"

            // cmd 60: cancel open receipt
            60 -> {
                isOpen = 0
                "0"
            }

            // cmd 255: read named parameter
            255 -> when (payloadStr.trim()) {
                "UNP" -> "0\t$currentUnp"
                "nLastFiscalDoc" -> "0\t$lastFiscalDoc"
                "FMnumber" -> "0\t$fmNumber"
                "Fiscalized" -> "0\t1"
                "CurrencyMode" -> "0\t$currencyMode"
                else -> "0\t"
            }

            // cmd 48: open receipt — set isOpen=1, increment number
            48 -> {
                isOpen = 1
                receiptNumber++
                "0"
            }

            // cmd 56: close receipt → returns ref with lastFiscalDoc
            56 -> {
                isOpen = 0
                lastFiscalDoc++
                val newUnp = "DY000600-A001-${"%07d".format(lastFiscalDoc)}"
                currentUnp = newUnp
                "0\t${lastFiscalDoc}"
            }

            else -> "0"
        }

        // For cmd 76, first field is IsOpen (not error code), format: ErrorCode\tIsOpen\t...
        // Actually Datecs cmd 76 returns: ErrorCode(0)\tIsOpen\tNumber\tItems\tAmount\tPayed
        // The requireOk() checks first field is 0 (success)
        val fixedResponse = when (command) {
            76 -> "0\t$isOpen\t$receiptNumber\t0\t0.00\t0.00"
            255 -> responseData  // already prefixed with "0\t"
            else -> responseData
        }

        val raw = DatecsText.encode(fixedResponse)
        return DatecsResponse(0x20, command, raw, byteArrayOf())
    }
}
