package com.beeloy.fiscal.daisy

import com.beeloy.fiscal.Command
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class DaisySmartStubTest {
    @Test fun productionActivationHardFails() {
        assertThrows(IllegalArgumentException::class.java) { DaisySmartStub("prod") }
        assertThrows(IllegalArgumentException::class.java) { DaisySmartStub("PROD") }
    }

    @Test fun catalogHasDeterministicSuccessAndFailureScenarios() {
        val driver = DaisySmartStub("dev")
        val success = driver.execute(Command("op-1", "FISCAL_SALE", mapOf("payments" to listOf(mapOf("type" to "CASH")))))
        assertEquals("FISCALIZED", success.state)
        assertTrue(success.simulated)
        val decline = driver.execute(Command("op-2", "FISCAL_SALE", mapOf("scenario" to "card_decline")))
        assertEquals("CARD_DECLINED", decline.errorCode)
        val unknown = driver.execute(Command("op-3", "FISCAL_SALE", mapOf("scenario" to "timeout_after_execution")))
        assertEquals("FISCAL_RESULT_UNKNOWN", unknown.state)
    }

    @Test fun cardPaymentRequiresActivatedTerminal() {
        val driver = DaisySmartStub("dev", cardTerminalAvailable = false)
        val result = driver.execute(Command("op-card", "FISCAL_SALE", mapOf("payments" to listOf(mapOf("type" to "CARD")))))
        assertEquals("PAYMENT_TERMINAL_UNAVAILABLE", result.errorCode)
    }

    @Test fun finalDeviceLossBlocksAndBarcodeIsCorrelated() {
        val unavailable = DaisySmartStub("dev", fiscalDeviceAvailable = false)
        assertEquals("BLOCKED", unavailable.execute(Command("op", "FISCAL_SALE", emptyMap())).state)
        val barcode = DaisySmartStub("dev").execute(Command("scan", "BARCODE_SCAN", emptyMap()))
        assertEquals("BARCODE_READ", barcode.state)
        assertEquals("3800000000017", barcode.data["barcode"])
    }

    @Test fun unsupportedCommandFailsClosed() {
        val result = DaisySmartStub("dev").execute(Command("op", "FORMAT_DEVICE", emptyMap()))
        assertEquals("UNSUPPORTED_CAPABILITY", result.errorCode)
    }
}
