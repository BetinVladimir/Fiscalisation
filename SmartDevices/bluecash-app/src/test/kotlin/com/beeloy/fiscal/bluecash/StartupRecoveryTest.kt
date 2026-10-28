package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

// FU status response: field[0]=errorCode, field[1]=isOpen (0/1)
private fun statusOpen()   = DatecsResponse(0x20, 76, "0\t1\t".toByteArray(Charsets.ISO_8859_1), byteArrayOf())
private fun statusClosed() = DatecsResponse(0x20, 76, "0\t0\t".toByteArray(Charsets.ISO_8859_1), byteArrayOf())
private fun cancelOk()     = DatecsResponse(0x20, 60, byteArrayOf(), byteArrayOf())

private class StartupFiscalPort(
    private val script: ArrayDeque<() -> DatecsResponse>
) : DatecsFiscalPort {
    val calls = mutableListOf<Int>()
    override fun reachable() = true
    override fun execute(command: Int, payload: ByteArray): DatecsResponse {
        calls.add(command)
        return script.removeFirst().invoke()
    }
}

private fun noCard() = object : DatecsPaymentPort {
    override fun reachable() = true
    override fun purchaseEur(amountMinor: Long, operationId: String) =
        throw UnsupportedOperationException()
    override fun reverse(operationId: String) =
        throw UnsupportedOperationException()
}

private fun signer() = object : TransactionSigner {
    override val keyId = "test"
    override fun sign(hash: ByteArray) = hash
}

class StartupRecoveryTest {

    /** Dangling receipt: cmd76 reports open → cmd60 cancel must be sent. */
    @Test fun startupCancelsDanglingReceipt() {
        val port = StartupFiscalPort(ArrayDeque(listOf({ statusOpen() }, { cancelOk() })))
        val journal = SignedMemoryJournal(signer())
        val proc = BlueCashCommandProcessor(port, noCard(), journal)

        val cancelled = proc.startup("startup-op-1")

        assertTrue("startup must return true when a dangling receipt is cancelled", cancelled)
        // cmd 76 (status) then cmd 60 (cancel)
        assertEquals(listOf(76, DatecsPayloads.CMD_CANCEL_RECEIPT), port.calls)
        val events = journal.find("startup-op-1")
        assertTrue("STARTUP_RECEIPT_CANCELED must be journaled",
            events.any { it.type == "STARTUP_RECEIPT_CANCELED" })
    }

    /** No open receipt: startup must return false and send no cancel. */
    @Test fun startupDoesNothingWhenNoReceipt() {
        val port = StartupFiscalPort(ArrayDeque(listOf({ statusClosed() })))
        val journal = SignedMemoryJournal(signer())
        val proc = BlueCashCommandProcessor(port, noCard(), journal)

        val cancelled = proc.startup("startup-op-2")

        assertFalse("startup must return false when no dangling receipt", cancelled)
        assertEquals(listOf(76), port.calls)
        assertTrue("no journal events for clean startup", journal.find("startup-op-2").isEmpty())
    }

    /** Cancel fails: startup returns false but journals STARTUP_RECEIPT_CANCEL_FAILED. */
    @Test fun startupHandlesCancelFailure() {
        val port = StartupFiscalPort(ArrayDeque(listOf(
            { statusOpen() },
            { throw DatecsFailure.Unknown("DATECS_TIMEOUT") }
        )))
        val journal = SignedMemoryJournal(signer())
        val proc = BlueCashCommandProcessor(port, noCard(), journal)

        val cancelled = proc.startup("startup-op-3")

        assertFalse("startup returns false on cancel failure", cancelled)
        assertTrue("STARTUP_RECEIPT_CANCEL_FAILED must be journaled",
            journal.find("startup-op-3").any { it.type == "STARTUP_RECEIPT_CANCEL_FAILED" })
    }
}
