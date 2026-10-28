package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * VG-23: PaidMode mapping per vendor spec PM §4.15 and
 * docs/MVP1/fixtures/payment_slots_bc50.json.
 *
 * BC50_DEFAULT: CASH→0, CARD→1, CHEQUE→3, VOUCHER→4, COUPON→5.
 * NHIF/DEFERRED/INTERNAL_CONSUMPTION → PAYMENT_TYPE_UNSUPPORTED (null).
 * Validation must happen BEFORE card I/O (invariant BUG-022).
 */
class PaymentModeMappingTest {

    // ─── VG-23.1: PaymentSlotMatchesVendorTable ───────────────────────────────
    // Fixture: docs/MVP1/fixtures/payment_slots_bc50.json

    @Test fun PaymentSlotMatchesVendorTable() {
        assertEquals("CASH must be slot 0",  0, paymentSlot(BC50Profile.DEFAULT, "CASH"))
        assertEquals("CARD must be slot 1",  1, paymentSlot(BC50Profile.DEFAULT, "CARD"))
        assertEquals("CHEQUE must be slot 3", 3, paymentSlot(BC50Profile.DEFAULT, "CHEQUE"))
        assertEquals("VOUCHER must be slot 4", 4, paymentSlot(BC50Profile.DEFAULT, "VOUCHER"))
        assertEquals("COUPON must be slot 5",  5, paymentSlot(BC50Profile.DEFAULT, "COUPON"))
    }

    // ─── VG-23.2: ChequeIsNotDebitCardSlot ────────────────────────────────────
    // Slot 2 = debit card in the vendor default slot table; CHEQUE must be slot 3.

    @Test fun ChequeIsNotDebitCardSlot() {
        val slot = paymentSlot(BC50Profile.DEFAULT, "CHEQUE")
        assertNotEquals(
            "CHEQUE must not occupy debit-card slot 2",
            2,
            slot
        )
        assertEquals("CHEQUE must be slot 3 per PM §4.15", 3, slot)
    }

    // ─── VG-23.3: UnsupportedTypeRejectedBeforeCardCharge ─────────────────────
    // NHIF, DEFERRED, INTERNAL_CONSUMPTION must return null so the engine can
    // raise PAYMENT_TYPE_UNSUPPORTED before any card or fiscal I/O.

    @Test fun UnsupportedTypeRejectedBeforeCardCharge() {
        assertNull("NHIF must return null (PAYMENT_TYPE_UNSUPPORTED)",
            paymentSlot(BC50Profile.DEFAULT, "NHIF"))
        assertNull("DEFERRED must return null (PAYMENT_TYPE_UNSUPPORTED)",
            paymentSlot(BC50Profile.DEFAULT, "DEFERRED"))
        assertNull("INTERNAL_CONSUMPTION must return null (PAYMENT_TYPE_UNSUPPORTED)",
            paymentSlot(BC50Profile.DEFAULT, "INTERNAL_CONSUMPTION"))
    }
}
