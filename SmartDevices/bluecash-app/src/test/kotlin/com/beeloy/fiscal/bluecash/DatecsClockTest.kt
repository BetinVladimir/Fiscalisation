package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime

private val SOFIA = ZoneId.of("Europe/Sofia")

/** 2024-03-28 02:00 Sofia is the DST spring-forward boundary (EET→EEST). */
private fun sofiaTime(year: Int, month: Int, day: Int, hour: Int, min: Int, sec: Int = 0): ZonedDateTime =
    ZonedDateTime.of(year, month, day, hour, min, sec, 0, SOFIA)

class DatecsClockTest {

    // ---- parse() correctness ------------------------------------------------

    @Test fun parsesNoDstResponse() {
        val hostNow = sofiaTime(2024, 6, 15, 12, 0, 0)
        val result = DatecsClock.parse("15-06-24\t12:00:00\t0", hostNow)
        assertEquals(LocalDate.of(2024, 6, 15), result.businessDate)
        assertEquals(0L, result.driftSeconds)
        assertEquals(DriftStatus.OK, result.driftStatus)
    }

    @Test fun parsesDstResponse() {
        val hostNow = sofiaTime(2024, 7, 1, 10, 0, 0)
        val result = DatecsClock.parse("01-07-24\t10:00:00\t1", hostNow)
        assertEquals(LocalDate.of(2024, 7, 1), result.businessDate)
        assertEquals(DriftStatus.OK, result.driftStatus)
    }

    @Test fun businessDateIsSofia() {
        // FU reports 23:59:59, business date must be that calendar day in Sofia
        val hostNow = sofiaTime(2024, 6, 14, 23, 59, 59)
        val result = DatecsClock.parse("14-06-24\t23:59:59\t1", hostNow)
        assertEquals(LocalDate.of(2024, 6, 14), result.businessDate)
    }

    // ---- drift classification -----------------------------------------------

    @Test fun driftOkWhenUnder5Min() {
        val hostNow = sofiaTime(2024, 6, 15, 12, 0, 0)
        // FU is 4 minutes 59 seconds ahead
        val result = DatecsClock.parse("15-06-24\t12:04:59\t0", hostNow)
        assertEquals(DriftStatus.OK, result.driftStatus)
        assertEquals(299L, result.driftSeconds)
    }

    @Test fun driftWarnAtExactly5Min() {
        val hostNow = sofiaTime(2024, 6, 15, 12, 0, 0)
        val result = DatecsClock.parse("15-06-24\t12:05:00\t0", hostNow)
        assertEquals(DriftStatus.WARN, result.driftStatus)
        assertEquals(300L, result.driftSeconds)
    }

    @Test fun driftWarnFor6MinDrift() {
        val hostNow = sofiaTime(2024, 6, 15, 12, 0, 0)
        val result = DatecsClock.parse("15-06-24\t12:06:00\t0", hostNow)
        assertEquals(DriftStatus.WARN, result.driftStatus)
    }

    @Test fun driftBlockAt2Hours() {
        val hostNow = sofiaTime(2024, 6, 15, 12, 0, 0)
        val result = DatecsClock.parse("15-06-24\t14:00:00\t0", hostNow)
        assertEquals(DriftStatus.BLOCK, result.driftStatus)
        assertEquals(7200L, result.driftSeconds)
    }

    @Test fun driftBlockWhenFuBehindBy3Hours() {
        val hostNow = sofiaTime(2024, 6, 15, 15, 0, 0)
        val result = DatecsClock.parse("15-06-24\t12:00:00\t0", hostNow)
        assertEquals(DriftStatus.BLOCK, result.driftStatus)
        assertEquals(-10800L, result.driftSeconds)
    }

    // ---- no host-clock fallback ---------------------------------------------

    @Test fun hostClockNotUsedAsFallback() {
        // Parse must use FU time, not host time; drift must reflect the difference
        val fuAheadBy10Min = sofiaTime(2024, 6, 15, 12, 10, 0)
        val hostNow = sofiaTime(2024, 6, 15, 12, 0, 0)
        val result = DatecsClock.parse("15-06-24\t12:10:00\t0", hostNow)
        assertEquals(fuAheadBy10Min.toInstant(), result.deviceTime.toInstant())
        assertEquals(600L, result.driftSeconds)
    }

    // ---- parse errors -------------------------------------------------------

    @Test(expected = ClockParseException::class)
    fun throwsOnMissingFields() {
        DatecsClock.parse("15-06-24\t12:00:00", sofiaTime(2024, 6, 15, 12, 0))
    }

    @Test(expected = ClockParseException::class)
    fun throwsOnBadDateFormat() {
        DatecsClock.parse("2024-06-15\t12:00:00\t0", sofiaTime(2024, 6, 15, 12, 0))
    }

    @Test(expected = ClockParseException::class)
    fun throwsOnNonIntDst() {
        DatecsClock.parse("15-06-24\t12:00:00\tX", sofiaTime(2024, 6, 15, 12, 0))
    }
}
