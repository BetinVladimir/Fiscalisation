package com.beeloy.fiscal.bluecash

import java.time.LocalDate
import java.time.LocalDateTime
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeParseException

/** cmd62 (0x3E) — read FU date/time.  cmd61 (set time) is NOT available on BC-50. */
internal const val CMD_READ_CLOCK = 62

/** Warn when FU clock drift exceeds this threshold. */
const val DRIFT_WARN_SECONDS = 300L   // 5 minutes

/** Block card/fiscal I/O when drift exceeds this threshold. */
const val DRIFT_BLOCK_SECONDS = 7200L // 2 hours

private val SOFIA = ZoneId.of("Europe/Sofia")
private val FU_DATE_FMT = DateTimeFormatter.ofPattern("dd-MM-yy")
private val FU_TIME_FMT = DateTimeFormatter.ofPattern("HH:mm:ss")

/**
 * Result from a single FU clock read.
 *
 * @param deviceTime  FU wall clock expressed in Europe/Sofia (DST applied).
 * @param driftSeconds  `deviceTime - hostNow` in whole seconds (positive = FU ahead).
 * @param businessDate  Calendar date at [deviceTime] in Europe/Sofia (midnight-aware).
 * @param driftStatus  DRIFT_OK / DRIFT_WARN / DRIFT_BLOCK.
 */
data class ClockResult(
    val deviceTime: ZonedDateTime,
    val driftSeconds: Long,
    val businessDate: LocalDate,
    val driftStatus: DriftStatus,
)

enum class DriftStatus { OK, WARN, BLOCK }

/** Thrown when the FU response cannot be parsed. */
class ClockParseException(msg: String) : Exception(msg)

object DatecsClock {

    /**
     * Parse a cmd62 tab-separated response and compute drift against [hostNow].
     *
     * The FU sends: `DD-MM-YY\tHH:MM:SS\tDST` where DST is 0 or 1.
     * We interpret the date/time as Europe/Sofia with the DST flag applied.
     *
     * @param fuResponse  raw text from [DatecsResponse.data] decoded as CP1251.
     * @param hostNow     caller-supplied host clock (injected for testability; never used as fallback).
     * @throws ClockParseException if the response cannot be parsed.
     */
    fun parse(fuResponse: String, hostNow: ZonedDateTime): ClockResult {
        val parts = fuResponse.trimEnd().split("\t")
        if (parts.size < 3) throw ClockParseException("CMD62: expected 3 tab-separated fields, got ${parts.size}")

        val datePart = parts[0].trim()
        val timePart = parts[1].trim()
        val dstFlag  = parts[2].trim().toIntOrNull()
            ?: throw ClockParseException("CMD62: DST field not an integer: '${parts[2]}'")

        val localDt: LocalDateTime = try {
            val date = LocalDate.parse(datePart, FU_DATE_FMT)
            val time = java.time.LocalTime.parse(timePart, FU_TIME_FMT)
            LocalDateTime.of(date, time)
        } catch (e: DateTimeParseException) {
            throw ClockParseException("CMD62: cannot parse date/time '$datePart $timePart': ${e.message}")
        }

        // Resolve the FU local time to a ZonedDateTime in Europe/Sofia.
        // When DST=1, force DST overlap resolution to the summer (DST) instant.
        val deviceTime: ZonedDateTime = if (dstFlag == 1) {
            localDt.atZone(SOFIA).withLaterOffsetAtOverlap()
        } else {
            localDt.atZone(SOFIA).withEarlierOffsetAtOverlap()
        }

        val hostSofia = hostNow.withZoneSameInstant(SOFIA)
        val driftSeconds = java.time.Duration.between(hostSofia.toInstant(), deviceTime.toInstant()).seconds
        val absDrift = kotlin.math.abs(driftSeconds)

        val driftStatus = when {
            absDrift >= DRIFT_BLOCK_SECONDS -> DriftStatus.BLOCK
            absDrift >= DRIFT_WARN_SECONDS  -> DriftStatus.WARN
            else                            -> DriftStatus.OK
        }

        return ClockResult(
            deviceTime   = deviceTime,
            driftSeconds = driftSeconds,
            businessDate = deviceTime.toLocalDate(),
            driftStatus  = driftStatus,
        )
    }
}
