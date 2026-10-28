package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * SMS-03: SmartFiscalPort — request body shape, typed error mapping.
 * Numbers from fixture: docs/MVP1/fixtures/smart_protocol_examples.json
 * Golden totals: item1=1200¢ (1234-34), item2=167¢, total=1367¢.
 */
class SMS03Test {

    // Fixture plan matching sale_cash.request exactly.
    private fun saleCashPlan() = SmartReceiptPlan(
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

    private fun capturePort(): Pair<SmartFiscalPort, MutableList<String>> {
        val captured = mutableListOf<String>()
        val port = SmartFiscalPort(
            send = { json ->
                captured += json
                SmartReply.Ok(
                    data = """{"docNum":"0042"}""",
                    messages = emptyList(),
                    seq = 1,
                )
            },
        )
        return port to captured
    }

    // Helpers for semantic JSON field extraction (no org.json).
    private fun jsonField(json: String, key: String): String? =
        Regex(""""$key"\s*:\s*("(?:[^"\\]|\\.)*"|-?\d+(?:\.\d+)?|true|false|null)""")
            .find(json)?.groupValues?.get(1)?.removeSurrounding("\"")

    private fun jsonArrayCount(json: String, marker: String): Int =
        Regex(Regex.escape(marker)).findAll(json).count()

    @Test fun `sale_cash request body is JSON-equivalent to fixture sale_cash_request`() {
        val (port, captured) = capturePort()
        port.sale(saleCashPlan())

        assertEquals("Exactly one HTTP call made", 1, captured.size)
        val body = captured[0]

        // Top-level structure
        assertEquals("PrintFiscalReceipt", jsonField(body, "command"))
        assertEquals("bg", jsonField(body, "errorsLocale"))
        assertEquals("1", jsonField(body, "seq"))

        // data fields
        assertEquals("DY000600-OP01-0000001", jsonField(body, "UNP"))
        assertEquals("op-0001", jsonField(body, "refNum"))
        assertEquals("20", jsonField(body, "opCode"))
        assertEquals("9999", jsonField(body, "opPassword"))

        // items — verify both items present by name
        assertTrue("Тест present in body", body.contains("Тест"))
        assertTrue("Тест 2 present in body", body.contains("Тест 2"))

        // item 1 prices
        assertTrue("price 12.34 present", body.contains("12.34"))
        assertTrue("qty 1.000 present", body.contains("1.000"))
        assertTrue("discountValue 0.34 present", body.contains("0.34"))
        assertTrue("discountType amount present", body.contains("amount"))
        assertTrue("vatGroup Б present", body.contains("Б"))

        // item 2
        assertTrue("price 3.33 present", body.contains("3.33"))
        assertTrue("qty 0.500 present", body.contains("0.500"))

        // payment
        assertTrue("payTypeCode 0 present", body.contains("\"payTypeCode\":0"))
        assertTrue("payment amount 13.67 present", body.contains("13.67"))

        // exactly 2 items and 1 payment
        assertEquals("Two 'type':'item' entries", 2, jsonArrayCount(body, "\"type\":\"item\""))
        assertEquals("One 'type':'payment' entry", 1, jsonArrayCount(body, "\"type\":\"payment\""))
    }

    @Test fun `Latin vatGroup B is rejected before any network call`() {
        val captured = mutableListOf<String>()
        val port = SmartFiscalPort(send = { json -> captured += json; SmartReply.NotSent })
        val plan = SmartReceiptPlan(
            unp = "DY000600-OP01-0000001",
            operationId = "op-x",
            opCode = 20,
            opPassword = "9999",
            items = listOf(SmartSaleItem("Item", priceCents = 1367, qtyMilli = 1000, vatGroup = "B")),
            payments = listOf(SmartSalePayment(0, 1367)),
        )
        val result = port.sale(plan)
        assertEquals("Expected TAX_GROUP_LATIN error", DeviceResult.Error("TAX_GROUP_LATIN"), result)
        assertEquals("No network call made", 0, captured.size)
    }

    @Test fun `ok false code 1008 maps to Rejected`() {
        val port = SmartFiscalPort(send = { SmartReply.Rejected(1008, listOf("Bad operator or password.")) })
        val result = port.sale(saleCashPlan())
        assertTrue("Expected Rejected, got $result", result is DeviceResult.Rejected)
        assertEquals(1008, (result as DeviceResult.Rejected).code)
    }

    @Test fun `ok false code 1000 maps to Rejected`() {
        val port = SmartFiscalPort(send = { SmartReply.Rejected(1000, listOf("Bad JSON syntax")) })
        val result = port.sale(saleCashPlan())
        assertTrue(result is DeviceResult.Rejected)
        assertEquals(1000, (result as DeviceResult.Rejected).code)
    }

    @Test fun `SmartReply Unknown maps to DeviceResult Unknown`() {
        val port = SmartFiscalPort(send = { SmartReply.Unknown })
        val result = port.sale(saleCashPlan())
        assertEquals(DeviceResult.Unknown, result)
    }

    @Test fun `SmartReply NotSent maps to DeviceResult NotSent`() {
        val port = SmartFiscalPort(send = { SmartReply.NotSent })
        val result = port.sale(saleCashPlan())
        assertEquals(DeviceResult.NotSent, result)
    }
}
