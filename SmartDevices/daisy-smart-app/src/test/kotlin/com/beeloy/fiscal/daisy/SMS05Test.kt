package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * SMS-05: SmartFiscalPort — mixed payment flow via PaymentBridge.
 * Golden totals: CASH 500¢ + CARD 867¢ = 1367¢ (invariant 9).
 * SMS-05.1: REJECTED → fiscalCalls==0; SUCCESS → fiscalCalls==1
 * SMS-05.2: card OK, fiscal FAILED → FiscalPending, refundCalls==0
 */
class SMS05Test {

    private fun mixedPlan() = SmartReceiptPlan(
        unp = "DY000600-OP01-0000001",
        operationId = "op-mixed-01",
        opCode = 20,
        opPassword = "9999",
        items = listOf(SmartSaleItem("Тест", 1367, 1000, "Б")),
        payments = listOf(
            SmartSalePayment(payTypeCode = 0, amountCents = 500),   // CASH 5.00
            SmartSalePayment(payTypeCode = 1, amountCents = 867),   // CARD 8.67
        ),
    )

    @Test fun `CARD REJECTED means zero fiscal calls`() {
        var fiscalCalls = 0
        val port = SmartFiscalPort(send = { _ ->
            fiscalCalls++
            SmartReply.Ok(data = """{"docNum":"1"}""", messages = emptyList(), seq = 1)
        })
        val bridge = object : PaymentBridge {
            override fun pay(operationId: String, amountCents: Long, bank: String) = PaymentResult.Rejected
            override fun refund(referenceId: String) = PaymentResult.RefundManualRequired
        }
        val result = port.paySale(mixedPlan(), 867, bridge, "dsk")
        assertEquals("fiscalCalls==0 when card rejected", 0, fiscalCalls)
        assertTrue("Result is CardRejected", result is SaleFlowResult.CardRejected)
    }

    @Test fun `CARD SUCCESS means fiscal called once`() {
        var fiscalCalls = 0
        val port = SmartFiscalPort(send = { _ ->
            fiscalCalls++
            SmartReply.Ok(data = """{"docNum":"42"}""", messages = emptyList(), seq = 1)
        })
        val bridge = object : PaymentBridge {
            override fun pay(operationId: String, amountCents: Long, bank: String) = PaymentResult.Committed("RRN-001")
            override fun refund(referenceId: String) = PaymentResult.RefundManualRequired
        }
        val result = port.paySale(mixedPlan(), 867, bridge, "dsk")
        assertEquals("fiscalCalls==1 when card SUCCESS", 1, fiscalCalls)
        assertTrue("Result is Done", result is SaleFlowResult.Done)
    }

    @Test fun `CARD OK fiscal FAILED returns FiscalPending with zero refund calls`() {
        var refundCalls = 0
        val port = SmartFiscalPort(send = { SmartReply.Rejected(1000, listOf("fiscal error")) })
        val bridge = object : PaymentBridge {
            override fun pay(operationId: String, amountCents: Long, bank: String) =
                PaymentResult.Committed("RRN-002")
            override fun refund(referenceId: String): PaymentResult {
                refundCalls++
                return PaymentResult.RefundManualRequired
            }
        }
        val result = port.paySale(mixedPlan(), 867, bridge, "dsk")
        assertEquals("refundCalls==0 after fiscal failure", 0, refundCalls)
        assertTrue("Result is FiscalPending, got $result", result is SaleFlowResult.FiscalPending)
    }

    @Test fun `CARD OK fiscal UNKNOWN returns FiscalPending with zero refund calls`() {
        var refundCalls = 0
        val port = SmartFiscalPort(send = { SmartReply.Unknown })
        val bridge = object : PaymentBridge {
            override fun pay(operationId: String, amountCents: Long, bank: String) =
                PaymentResult.Committed("RRN-003")
            override fun refund(referenceId: String): PaymentResult {
                refundCalls++
                return PaymentResult.RefundManualRequired
            }
        }
        val result = port.paySale(mixedPlan(), 867, bridge, "dsk")
        assertEquals("refundCalls==0 when fiscal unknown", 0, refundCalls)
        assertTrue("Result is FiscalPending on Unknown, got $result", result is SaleFlowResult.FiscalPending)
    }

    @Test fun `CARD NOT_COMMITTED means zero fiscal calls`() {
        var fiscalCalls = 0
        val port = SmartFiscalPort(send = { _ ->
            fiscalCalls++
            SmartReply.Ok(data = """{"docNum":"5"}""", messages = emptyList(), seq = 1)
        })
        val bridge = object : PaymentBridge {
            override fun pay(operationId: String, amountCents: Long, bank: String) = PaymentResult.NotCommitted
            override fun refund(referenceId: String) = PaymentResult.RefundManualRequired
        }
        val result = port.paySale(mixedPlan(), 867, bridge, "dsk")
        assertEquals("fiscalCalls==0 when card not committed", 0, fiscalCalls)
        assertTrue("Result is CardRejected when not committed", result is SaleFlowResult.CardRejected)
    }
}
