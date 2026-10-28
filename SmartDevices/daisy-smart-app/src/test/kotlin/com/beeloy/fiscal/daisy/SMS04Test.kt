package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * SMS-04: SmartFiscalPort — one call per sale, docNum-absent handling.
 * Golden totals: 1200¢ + 167¢ = 1367¢ (invariant 9).
 */
class SMS04Test {

    private fun twoLinePlan() = SmartReceiptPlan(
        unp = "DY000600-OP01-0000001",
        operationId = "op-0001",
        opCode = 20,
        opPassword = "9999",
        items = listOf(
            SmartSaleItem("Тест", priceCents = 1234, qtyMilli = 1000, vatGroup = "Б", discountCents = 34),
            SmartSaleItem("Тест 2", priceCents = 333, qtyMilli = 500, vatGroup = "Б"),
        ),
        payments = listOf(SmartSalePayment(payTypeCode = 0, amountCents = 1367)),
    )

    @Test fun `ReceiptPlan 1367 CASH produces exactly one PrintFiscalReceipt call`() {
        val callCount = arrayOf(0)
        val port = SmartFiscalPort(send = { json ->
            callCount[0]++
            assertTrue("Command must be PrintFiscalReceipt", json.contains("PrintFiscalReceipt"))
            SmartReply.Ok(data = """{"docNum":"55"}""", messages = emptyList(), seq = 1)
        })
        val result = port.sale(twoLinePlan())
        assertEquals("httpCalls==1", 1, callCount[0])
        assertTrue("Expected Committed", result is DeviceResult.Committed)
    }

    @Test fun `UNP and refNum come from the plan`() {
        var captured = ""
        val port = SmartFiscalPort(send = { json ->
            captured = json
            SmartReply.Ok(data = """{"docNum":"1"}""", messages = emptyList(), seq = 1)
        })
        port.sale(twoLinePlan())
        assertTrue("UNP from plan", captured.contains("DY000600-OP01-0000001"))
        assertTrue("refNum from plan", captured.contains("op-0001"))
    }

    @Test fun `Cyrillic vatGroup Б is accepted`() {
        val port = SmartFiscalPort(send = { SmartReply.Ok(data = """{"docNum":"2"}""", messages = emptyList(), seq = 1) })
        val result = port.sale(twoLinePlan())
        assertTrue("Cyrillic vatGroup accepted, got $result", result is DeviceResult.Committed)
    }

    @Test fun `Latin vatGroup B triggers TAX_GROUP_LATIN before network`() {
        val callCount = arrayOf(0)
        val port = SmartFiscalPort(send = { callCount[0]++; SmartReply.NotSent })
        val plan = twoLinePlan().copy(
            items = listOf(SmartSaleItem("X", 1367, 1000, "B")),
        )
        val result = port.sale(plan)
        assertEquals(DeviceResult.Error("TAX_GROUP_LATIN"), result)
        assertEquals("No network call on TAX_GROUP_LATIN", 0, callCount[0])
    }

    @Test fun `ok true without docNum in PROD returns Unknown`() {
        val port = SmartFiscalPort(
            send = { SmartReply.Ok(data = """{"ok":true}""", messages = emptyList(), seq = 1) },
            isLab = false,
        )
        val result = port.sale(twoLinePlan())
        // Invariant 6: no synthetic docNum; must return Unknown in PROD.
        assertEquals(DeviceResult.Unknown, result)
    }

    @Test fun `ok true without docNum in LAB returns Committed with noDocNumber flag`() {
        val port = SmartFiscalPort(
            send = { SmartReply.Ok(data = """{"ok":true}""", messages = emptyList(), seq = 1) },
            isLab = true,
        )
        val result = port.sale(twoLinePlan())
        assertTrue("Expected Committed in LAB, got $result", result is DeviceResult.Committed)
        assertTrue("noDocNumber flag set", (result as DeviceResult.Committed).noDocNumber)
    }
}
