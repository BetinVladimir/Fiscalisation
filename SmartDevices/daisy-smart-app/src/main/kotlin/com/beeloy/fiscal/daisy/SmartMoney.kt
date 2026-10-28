package com.beeloy.fiscal.daisy

import java.math.BigDecimal
import java.math.RoundingMode

/**
 * Money utilities for Smart S HTTP API.
 * Invariant 1: integer cents only; no float/double arithmetic.
 * Rounding: half-up (unit¢ × qty_milli + 500) / 1000.
 */
object SmartMoney {

    fun centsToJson(cents: Long): String =
        BigDecimal(cents).divide(BigDecimal(100), 2, RoundingMode.UNNECESSARY).toPlainString()

    fun qtyMilliToJson(milli: Long): String =
        BigDecimal(milli).divide(BigDecimal(1000), 3, RoundingMode.UNNECESSARY).toPlainString()

    fun lineCents(unitCents: Long, qtyMilli: Long, discountCents: Long = 0L): Long =
        (unitCents * qtyMilli + 500L) / 1000L - discountCents
}
