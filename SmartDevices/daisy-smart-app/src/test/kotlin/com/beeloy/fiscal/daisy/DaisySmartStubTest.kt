package com.beeloy.fiscal.daisy

import com.beeloy.fiscal.Command
import org.junit.Assert.assertEquals
import org.junit.Test

class DaisySmartStubTest {
    @Test fun debugStubHasDeterministicSaleResult() {
        val result = DaisySmartStub("dev").execute(Command("operation-1", "FISCAL_SALE", mapOf("scenario" to "success")))
        assertEquals("FISCALIZED", result.state)
        assertEquals(true, result.simulated)
    }
}
