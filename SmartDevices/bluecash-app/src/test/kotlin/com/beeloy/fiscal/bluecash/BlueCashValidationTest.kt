package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/** VG-06.3 — plan validation before I/O. */
class BlueCashValidationTest {

    private fun makeEngine() = BlueCashCommandProcessor(
        Fiscal(), Card(), SignedMemoryJournal(Signer())
    )

    // VG-06.3: invalid plan (total mismatch) must not charge card or open receipt
    @Test
    fun `invalid plan does not charge card`() {
        val fiscal = Fiscal()
        val card = Card()
        val engine = BlueCashCommandProcessor(fiscal, card, SignedMemoryJournal(Signer()))

        // Lines total: 13.67 × 1 = 13.67 (1367¢), payment: 10.00 (1000¢) → mismatch
        val sale = BlueCashSale(
            "op-invalid",
            "DY000600-A001-0000001",
            1, "0000", 1,
            listOf(FiscalLine("Coffee", 'B', "13.67", "1.000")),
            listOf(SalePayment("p1", "CARD", "10.00"))   // wrong total
        )
        val result = engine.sale(sale)
        // Should fail validation before I/O
        assertEquals("FAILED", result["state"])
        assertEquals(0, card.calls)            // no card charge
        assertTrue("No cmd 48 (open receipt) before validation fails",
            fiscal.calls.none { it.first == 48 })
    }

    @Test
    fun `empty lines are rejected before IO`() {
        val fiscal = Fiscal()
        val card = Card()
        val engine = BlueCashCommandProcessor(fiscal, card, SignedMemoryJournal(Signer()))
        val threw = try {
            engine.sale(
                BlueCashSale(
                    "op-empty",
                    "DY000600-A001-0000001",
                    1, "0000", 1,
                    emptyList(),
                    listOf(SalePayment("p1", "CARD", "2.65"))
                )
            )
            false
        } catch (e: IllegalArgumentException) { true }
        assertTrue("Empty lines should throw or fail before I/O", threw)
        assertEquals(0, card.calls)
        assertTrue(fiscal.calls.none { it.first == 48 })
    }

    @Test
    fun `valid plan with golden receipt executes without error`() {
        val fiscal = Fiscal()
        val card = Card()
        val engine = BlueCashCommandProcessor(fiscal, card, SignedMemoryJournal(Signer()))

        // Golden: 1200¢ + 167¢ = 1367¢; CASH 500¢ + CARD 867¢
        val sale = BlueCashSale(
            "op-golden",
            "DY000600-A001-0000001",
            1, "0000", 1,
            listOf(
                FiscalLine("Item A", 'B', "12.00", "1.000"),
                FiscalLine("Item B", 'B', "1.67", "1.000")
            ),
            listOf(
                SalePayment("p1", "CASH", "5.00"),
                SalePayment("p2", "CARD", "8.67")
            )
        )
        val result = engine.sale(sale)
        assertEquals("FISCALIZED", result["state"])
    }
}
