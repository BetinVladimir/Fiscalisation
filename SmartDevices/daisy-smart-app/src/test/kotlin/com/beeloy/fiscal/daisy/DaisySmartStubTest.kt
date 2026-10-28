package com.beeloy.fiscal.daisy

import com.beeloy.fiscal.Command
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

class DaisySmartStubTest {
    @Test fun debugStubHasDeterministicSaleResult() {
        val result = DaisySmartStub("dev").execute(Command("operation-1", "FISCAL_SALE", mapOf("scenario" to "success")))
        assertEquals("FISCALIZED", result.state)
        assertEquals(true, result.simulated)
    }

    @Test fun `PROD environment rejects stub instantiation`() {
        try {
            DaisySmartStub("prod")
            fail("Expected IllegalArgumentException for PROD environment")
        } catch (e: IllegalArgumentException) {
            assertTrue(
                "Message must mention PROD: ${e.message}",
                e.message?.contains("PROD", ignoreCase = true) == true ||
                e.message?.contains("forbidden", ignoreCase = true) == true
            )
        }
    }

    @Test fun `LAB mode stub executes FISCAL_SALE successfully`() {
        val stub = DaisySmartStub("lab")
        val result = stub.execute(Command("op-lab-1", "FISCAL_SALE", mapOf("scenario" to "success")))
        assertEquals("FISCALIZED", result.state)
        assertEquals(true, result.simulated)
    }

    @Test fun `stub FISCAL_RESULT_UNKNOWN on timeout_after_execution scenario`() {
        val result = DaisySmartStub("dev").execute(
            Command("op-2", "FISCAL_SALE", mapOf("scenario" to "timeout_after_execution"))
        )
        assertEquals("FISCAL_RESULT_UNKNOWN", result.state)
        assertEquals(true, result.simulated)
    }

    @Test fun `stub rejects unknown capability`() {
        val result = DaisySmartStub("dev").execute(Command("op-3", "UNKNOWN_CMD", emptyMap()))
        assertEquals("FAILED", result.state)
    }
}
