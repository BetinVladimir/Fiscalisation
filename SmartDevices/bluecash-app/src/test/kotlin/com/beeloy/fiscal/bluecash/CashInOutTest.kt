package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.math.BigDecimal

/**
 * VG-15: cmd 70 — cash in/out (§4.30): balance check before storno-cash,
 * and UNKNOWN resolution via cmd 70 totals.
 */
class CashInOutTest {

    // ─── VG-15.1: StornoCashRequiresAvailableBalance ──────────────────────────
    // When performing a cash-return storno, the requested amount must not exceed
    // the current cash balance reported by cmd 70.

    @Test fun StornoCashRequiresAvailableBalance_sufficientBalance() {
        // balance 50.00, cash-out 13.67 — must succeed
        DatecsPayloads.requireCashAvailable("13.67", BigDecimal("50.00"))
        // no exception = sufficient balance
    }

    @Test fun StornoCashRequiresAvailableBalance_exactBalance() {
        // balance = amount — must succeed (equal is OK)
        DatecsPayloads.requireCashAvailable("13.67", BigDecimal("13.67"))
    }

    @Test fun StornoCashRequiresAvailableBalance_insufficientBalance() {
        // balance 5.00, cash-out 13.67 — must throw CASH_OUT_EXCEEDS_BALANCE
        try {
            DatecsPayloads.requireCashAvailable("13.67", BigDecimal("5.00"))
            fail("Must throw when cash-out exceeds balance")
        } catch (e: IllegalArgumentException) {
            assertTrue("Error must contain CASH_OUT_EXCEEDS_BALANCE, got: ${e.message}",
                e.message?.contains("CASH_OUT_EXCEEDS_BALANCE") == true)
        }
    }

    @Test fun StornoCashRequiresAvailableBalance_zeroBalance() {
        // balance 0.00, any cash-out must fail
        try {
            DatecsPayloads.requireCashAvailable("0.01", BigDecimal.ZERO)
            fail("Must throw when balance is zero")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message?.contains("CASH_OUT_EXCEEDS_BALANCE") == true)
        }
    }

    // ─── VG-15.2: CashOutUnknownResolvedByCmd70Totals ─────────────────────────
    // After a cash-out operation with UNKNOWN outcome, the cmd 70 totals response
    // can be used to determine whether the operation was actually recorded.

    @Test fun CashOutUnknownResolvedByCmd70Totals_parsesResponse() {
        // Simulate cmd 70 response: error=0, cashIn=150.00, cashOut=50.00, balance=100.00
        val responseData = "0\t150.00\t50.00\t100.00\t".toByteArray(Charsets.UTF_8)
        val response = DatecsResponse(0x20, 70, responseData, byteArrayOf())
        val result = DatecsPayloads.parseCashInOutResponse(response)
        assertNotNull("parseCashInOutResponse must return non-null for valid response", result)
        assertEquals("cashIn must be 150.00",  BigDecimal("150.00"), result!!.cashIn)
        assertEquals("cashOut must be 50.00",  BigDecimal("50.00"),  result.cashOut)
        assertEquals("cashBalance must be 100.00", BigDecimal("100.00"), result.cashBalance)
    }

    @Test fun CashOutUnknownResolvedByCmd70Totals_goldenAmounts() {
        // Golden amounts: cash balance 5.00 (500¢) — 8.67 (867¢) = negative, would fail;
        // but balance 13.67 (1367¢) - 8.67 (867¢) = 5.00 (500¢) remaining.
        val responseData = "0\t13.67\t8.67\t5.00\t".toByteArray(Charsets.UTF_8)
        val response = DatecsResponse(0x20, 70, responseData, byteArrayOf())
        val result = DatecsPayloads.parseCashInOutResponse(response)
        assertNotNull(result)
        assertEquals(BigDecimal("5.00"), result!!.cashBalance)
        // Confirm the balance check: another 5.00 cash-out is OK
        DatecsPayloads.requireCashAvailable("5.00", result.cashBalance)
        // But 5.01 would fail
        try {
            DatecsPayloads.requireCashAvailable("5.01", result.cashBalance)
            fail("5.01 must exceed remaining balance of 5.00")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message?.contains("CASH_OUT_EXCEEDS_BALANCE") == true)
        }
    }

    @Test fun CashOutUnknownResolvedByCmd70Totals_returnsNullOnError() {
        // If error code != 0, response must return null
        val responseData = "-110210\t".toByteArray(Charsets.UTF_8)
        val response = DatecsResponse(0x20, 70, responseData, byteArrayOf())
        assertNull("parseCashInOutResponse must return null on non-zero error code",
            DatecsPayloads.parseCashInOutResponse(response))
    }

    @Test fun CashOutUnknownResolvedByCmd70Totals_cashPayloadEncoding() {
        // cmd 70 request payload: Type 0=cash-in, 1=cash-out
        val cashInPayload = DatecsPayloads.cash(out = false, amount = "13.67")
        val cashOutPayload = DatecsPayloads.cash(out = true,  amount = "13.67")
        val cashInText  = DatecsText.decode(cashInPayload)
        val cashOutText = DatecsText.decode(cashOutPayload)
        assertEquals("cash-in type must be 0", "0", cashInText.split('\t').first())
        assertEquals("cash-out type must be 1", "1", cashOutText.split('\t').first())
    }
}
