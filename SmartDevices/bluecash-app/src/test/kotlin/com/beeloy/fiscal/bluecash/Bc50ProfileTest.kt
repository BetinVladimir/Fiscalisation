package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * VG-22: BC-50 hardware profile — commands listed in the Datecs PM as
 * "not used in BC-50" must never be emitted to the device.
 *
 * PM references: §4.1(33), §4.2(35), §4.10(47), §4.22(61), §4.24(63), §4.35(80), §4.54(106).
 */
class Bc50ProfileTest {

    // ─── VG-22.1: Bc50EmulatorRejectsUnsupportedCommands ─────────────────────
    // The constant set must contain exactly the 7 cmds documented as not-for-BC-50.

    @Test fun Bc50EmulatorRejectsUnsupportedCommands() {
        val expected = setOf(33, 35, 47, 61, 63, 80, 106)
        assertEquals(
            "BC50_UNSUPPORTED_CMDS must match the 7 commands listed in PM",
            expected,
            DatecsLimits.BC50_UNSUPPORTED_CMDS
        )
        // Each unsupported cmd must be present in the set.
        for (cmd in expected) {
            assertTrue("cmd $cmd must be in BC50_UNSUPPORTED_CMDS",
                cmd in DatecsLimits.BC50_UNSUPPORTED_CMDS)
        }
    }

    // ─── VG-22.2: EngineNeverEmitsUnsupportedCommands ────────────────────────
    // Run all normal engine scenarios and verify none of the unsupported
    // commands appear in the captured frame sequence.

    @Test fun EngineNeverEmitsUnsupportedCommands() {
        val signer = object : TransactionSigner {
            override val keyId = "k"
            override fun sign(hash: ByteArray) = hash
        }
        val capturedCmds = mutableListOf<Int>()
        val fiscal = object : DatecsFiscalPort {
            override fun reachable() = true
            override fun execute(command: Int, payload: ByteArray): DatecsResponse {
                capturedCmds += command
                val data = when (command) {
                    56   -> "0\tBON-001\t20240101\t123456".toByteArray()
                    else -> "0\t".toByteArray()
                }
                return DatecsResponse(0x20, command, data, byteArrayOf())
            }
        }
        val card = object : DatecsPaymentPort {
            override fun reachable() = true
            override fun purchaseEur(amountMinor: Long, operationId: String) =
                mapOf("approved" to "true", "rrn" to "R1")
            override fun reverse(operationId: String) = mapOf("approved" to "true")
        }
        val journal = SignedMemoryJournal(signer)
        val proc = BlueCashCommandProcessor(fiscal, card, journal)

        // sale scenario
        proc.sale(BlueCashSale(
            operationId  = "op-vc-001",
            unp          = "DY000600-OP01-0000001",
            operator     = 1,
            password     = "0000",
            till         = 1,
            lines        = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
            payments     = listOf(SalePayment("p1", "CASH", "13.67"))
        ))

        // z-report scenario
        proc.dailyReport("z-vc-001", zReport = true)

        // printer test scenario
        proc.printerTest("pt-vc-001")

        // reverse scenario — requires a completed sale first
        val storno = BlueCashReversal(
            operationId          = "rev-vc-001",
            originalOperationId  = "op-vc-001",
            operator             = 1,
            password             = "0000",
            till                 = 1,
            originalDocument     = DatecsStornoDocument(
                reason              = 0,
                documentNumber      = 1L,
                documentDateTime    = "24-04-19 08:36:27",
                fiscalMemoryNumber  = "02636571",
                originalUnp         = "DY000600-OP01-0000001"
            ),
            lines    = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
            payments = listOf(SalePayment("p1r", "CASH", "13.67"))
        )
        proc.reverse(storno)

        // None of the emitted commands should be in BC50_UNSUPPORTED_CMDS
        val violations = capturedCmds.filter { it in DatecsLimits.BC50_UNSUPPORTED_CMDS }
        assertTrue(
            "Engine emitted BC-50-unsupported commands: $violations",
            violations.isEmpty()
        )
    }
}
