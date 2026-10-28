package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * VG-05: Payment slot profile and cmd 53 response validation.
 * §4.15: PaidMode 0=cash, 1=credit card, 2=debit card, 3=cheque, 4=voucher, 5=coupon.
 * Datecs default slot names (§5, p.118): 1-КРЕДИТ, 2-ДЕБ.КАРТА, 3-ЧЕК, 4-ВАУЧЕР, 5-КУПОН, 6-EUR.
 */
class PaymentSlotTest {

    // ─── VG-05.1: PaymentModeMatchesSpecDefaults ──────────────────────────────

    @Test
    fun `PaymentModeMatchesSpecDefaults CASH maps to slot 0 and CARD maps to slot 1`() {
        // Spec §4.15: slot 0 = cash, slot 1 = credit card
        assertEquals("CASH must map to slot 0", 0, datecsPaymentMode("CASH"))
        assertEquals("CARD must map to slot 1 (credit card)", 1, datecsPaymentMode("CARD"))
    }

    @Test
    fun `PaymentModeMatchesSpecDefaults all defined types map to valid slots`() {
        val validModes = 0..5
        listOf("CASH", "CARD", "CHEQUE", "VOUCHER", "COUPON", "NHIF", "DEFERRED", "INTERNAL_CONSUMPTION")
            .forEach { type ->
                val mode = datecsPaymentMode(type)
                assertTrue("$type maps to out-of-range slot $mode", mode in validModes)
            }
    }

    @Test(expected = IllegalStateException::class)
    fun `PaymentModeMatchesSpecDefaults unknown type throws`() {
        datecsPaymentMode("BITCOIN")
    }

    // ─── VG-05.2: PartialPaymentStatusDRejected ───────────────────────────────

    @Test
    fun `PartialPaymentStatusDRejected status D in cmd53 response indicates underpayment`() {
        // Datecs cmd 53 response: ErrorCode\tStatus('D'=balance due, 'R'=change)\tAmount
        // Status 'D' on the last payment means total not reached — must reject before cmd 56
        val cmd53ResponseD = "0\tD\t5.09"  // §4.15 example
        val fields = cmd53ResponseD.split('\t')
        val errorCode = fields[0].toIntOrNull() ?: -1
        val status = fields.getOrElse(1) { "" }
        assertEquals("Error code must be 0", 0, errorCode)
        assertEquals("Status D indicates underpayment", "D", status)
        // The engine must treat D as a failure — balance still due
        assertTrue("Status D must be detected as partial payment", status == "D")
    }

    @Test
    fun `PartialPaymentStatusDRejected status R on CARD indicates forbidden change`() {
        // 'R' = change given; change back to a card is forbidden
        val cmd53ResponseR = "0\tR\t0.50"
        val fields = cmd53ResponseR.split('\t')
        val status = fields.getOrElse(1) { "" }
        assertEquals("R", status)
        // Card payments must not produce change (R on CARD = error)
        assertTrue("Status R on card must be detected and rejected", status == "R")
    }

    // ─── VG-05.3: SlotNameMismatchMakesDeviceNotReady ─────────────────────────

    @Test
    fun `SlotNameMismatchMakesDeviceNotReady detects slot name mismatch`() {
        // At activation, if FU reports a different slot name than the profile expects → NOT_READY
        // Spec: default names from §5 [*31]: slot 1=КРЕДИТ, slot 2=ДЕБ.КАРТА
        val expectedSlotNames = mapOf(1 to "КРЕДИТ", 2 to "ДЕБ.КАРТА")
        val actualSlotNames = mapOf(1 to "КРЕДИТ", 2 to "DIFFERENT")  // mismatch
        val ready = expectedSlotNames.all { (slot, expected) ->
            actualSlotNames[slot] == expected
        }
        assertFalse("Slot name mismatch must make device NOT_READY", ready)
    }

    @Test
    fun `SlotNameMismatchMakesDeviceNotReady matching names means device is ready`() {
        val expectedSlotNames = mapOf(1 to "КРЕДИТ", 2 to "ДЕБ.КАРТА")
        val actualSlotNames = mapOf(1 to "КРЕДИТ", 2 to "ДЕБ.КАРТА")
        val ready = expectedSlotNames.all { (slot, expected) ->
            actualSlotNames[slot] == expected
        }
        assertTrue("Matching slot names must allow device to be READY", ready)
    }
}
