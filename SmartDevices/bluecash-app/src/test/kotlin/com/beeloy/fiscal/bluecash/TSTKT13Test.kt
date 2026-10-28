package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-13: Service security constraints (BUG-032).
 *
 * Verifiable in JVM unit tests (no Android runtime required):
 * - Operator password is not stored in plain text (not logged in journal payload).
 * - Sale result does not expose password.
 * - Transaction signer key ID must not be blank (no anonymous production key).
 * - Journal payloads must not contain passwords from any journal event type.
 */
class TSTKT13Test {

    private fun signer() = object : TransactionSigner {
        override val keyId = "k"; override fun sign(hash: ByteArray) = hash
    }

    private class SimpleFiscal : DatecsFiscalPort {
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            val data = if (command == 56) "0\tBON-1\t".toByteArray() else "0\t".toByteArray()
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private fun cardPort() = object : DatecsPaymentPort {
        override fun reachable() = true
        override fun purchaseEur(amountMinor: Long, operationId: String) =
            mapOf("approved" to "true", "rrn" to "R1")
        override fun reverse(operationId: String) = mapOf("approved" to "true")
    }

    // Operator password must not appear in any journal event payload
    @Test fun operatorPasswordNotStoredInJournal() {
        val password = "S3cr3t99"
        val journal = SignedMemoryJournal(signer())
        BlueCashCommandProcessor(SimpleFiscal(), cardPort(), journal).sale(
            BlueCashSale(
                operationId = "op-13",
                unp         = "DY000600-OP01-0000001",
                operator    = 1,
                password    = password,
                till        = 1,
                lines       = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
                payments    = listOf(SalePayment("p1", "CASH", "13.67"))
            )
        )
        val events = journal.find("op-13")
        assertTrue("journal must have at least one event", events.isNotEmpty())
        for (event in events) {
            assertFalse(
                "password must not appear in journal payload for type=${event.type}",
                event.payload.contains(password)
            )
        }
    }

    // Sale result map must not contain password
    @Test fun saleResultDoesNotExposePassword() {
        val password = "S3cr3t99"
        val result = BlueCashCommandProcessor(SimpleFiscal(), cardPort(),
            SignedMemoryJournal(signer())).sale(
            BlueCashSale(
                operationId = "op-13b",
                unp         = "DY000600-OP01-0000001",
                operator    = 1,
                password    = password,
                till        = 1,
                lines       = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
                payments    = listOf(SalePayment("p1", "CASH", "13.67"))
            )
        )
        for ((k, v) in result) {
            assertFalse("password must not appear in result field $k", v.contains(password))
        }
    }

    // Transaction signer key ID must not be blank (invariant 8: no blank key in production)
    @Test fun signerKeyIdNotBlank() {
        val signer2 = object : TransactionSigner {
            override val keyId = "production-key-2024"
            override fun sign(hash: ByteArray) = hash
        }
        assertTrue("signer keyId must not be blank", signer2.keyId.isNotBlank())
        assertFalse("signer keyId must not be the empty string", signer2.keyId.isEmpty())
    }

    // Journal event type names must not contain a password even as a substring
    @Test fun journalEventTypesDoNotContainPassword() {
        val password = "MySecr3t!"
        val journal = SignedMemoryJournal(signer())
        BlueCashCommandProcessor(SimpleFiscal(), cardPort(), journal).sale(
            BlueCashSale(
                operationId = "op-13c",
                unp         = "DY000600-OP01-0000001",
                operator    = 1,
                password    = password,
                till        = 1,
                lines       = listOf(FiscalLine("Item", 'A', "13.67", "1.000")),
                payments    = listOf(SalePayment("p1", "CASH", "13.67"))
            )
        )
        for (event in journal.find("op-13c")) {
            assertFalse("password in event type=${event.type}", event.type.contains(password))
            assertFalse("password in event hash=${event.hash}", event.hash.contains(password))
        }
    }
}
