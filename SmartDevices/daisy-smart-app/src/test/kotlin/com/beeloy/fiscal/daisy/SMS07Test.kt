package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

/**
 * SMS-07: SmartFiscalPort.payments() — GetPayments on startup, CASH slot detection, caching.
 * Fixture: docs/MVP1/fixtures/payment_slots_smart.json
 * CASH = payTypeNap==0 AND currencyRate==1  (slot id=0 "В БРОЙ").
 */
class SMS07Test {

    // JSON array matching the payment_slots_smart.json fixture.
    private val slotsJson = """
        [
          {"id":0,"name":"В БРОЙ","currencyRate":1,"payTypeNap":0},
          {"id":1,"name":"С ЧЕК","currencyRate":1,"payTypeNap":1},
          {"id":2,"name":"Кредит","currencyRate":1,"payTypeNap":7},
          {"id":3,"name":"USD","currencyRate":0,"payTypeNap":0},
          {"id":4,"name":"EURO","currencyRate":0,"payTypeNap":0}
        ]
    """.trimIndent()

    private fun getPaymentsReply() = SmartReply.Ok(
        data = slotsJson,
        messages = emptyList(),
        seq = 1,
    )

    @Test fun `payments() sends GetPayments command`() {
        var captured = ""
        val port = SmartFiscalPort(send = { json -> captured = json; getPaymentsReply() })
        port.payments()
        assertTrue("Command must be GetPayments, got: $captured", captured.contains("GetPayments"))
    }

    @Test fun `payments() parses all 5 slots from fixture`() {
        val port = SmartFiscalPort(send = { getPaymentsReply() })
        val slots = port.payments()
        assertEquals("5 slots parsed", 5, slots.size)
        assertEquals(0, slots[0].id)
        assertEquals("В БРОЙ", slots[0].name)
        assertEquals(1, slots[0].currencyRate)
        assertEquals(0, slots[0].payTypeNap)
    }

    @Test fun `cashPayTypeCode returns id 0 for В БРОЙ slot`() {
        val port = SmartFiscalPort(send = { getPaymentsReply() })
        assertEquals("CASH slot id must be 0", 0, port.cashPayTypeCode())
    }

    @Test fun `USD slot id 3 is not CASH (currencyRate==0)`() {
        val port = SmartFiscalPort(send = { getPaymentsReply() })
        val slots = port.payments()
        val usd = slots.first { it.name == "USD" }
        assertTrue("USD has currencyRate==0", usd.currencyRate == 0)
        // cashPayTypeCode must not return USD
        assertEquals("CASH is not USD slot", 0, port.cashPayTypeCode())
    }

    @Test fun `payments() is cached — GetPayments called only once`() {
        var callCount = 0
        val port = SmartFiscalPort(send = { json ->
            if (json.contains("GetPayments")) callCount++
            getPaymentsReply()
        })
        port.payments()
        port.payments()
        port.cashPayTypeCode()
        assertEquals("GetPayments called exactly once (cached)", 1, callCount)
    }

    @Test fun `no matching CASH slot throws PAYMENT_TYPE_UNSUPPORTED`() {
        // All slots have currencyRate==0 — no CASH slot
        val noCache = """[{"id":0,"name":"USD","currencyRate":0,"payTypeNap":0}]"""
        val port = SmartFiscalPort(send = { SmartReply.Ok(data = noCache, messages = emptyList(), seq = 1) })
        try {
            port.cashPayTypeCode()
            fail("Expected IllegalStateException for missing CASH slot")
        } catch (e: IllegalStateException) {
            assertTrue(
                "Message must mention PAYMENT_TYPE_UNSUPPORTED: ${e.message}",
                e.message?.contains("PAYMENT_TYPE_UNSUPPORTED") == true,
            )
        }
    }
}
