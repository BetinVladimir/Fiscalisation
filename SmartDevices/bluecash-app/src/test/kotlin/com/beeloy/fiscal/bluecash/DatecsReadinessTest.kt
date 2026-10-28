package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test
import java.time.ZonedDateTime
import java.time.ZoneId

/**
 * VG-10: DatecsReadiness — parameter reading (cmd 255×N, cmd 74, cmd 68) and
 * blocking/warning rules (napBlockAt, zNeeded, reportsLeft).
 */
class DatecsReadinessTest {

    private val SOFIA = ZoneId.of("Europe/Sofia")

    /** A port that returns pre-programmed responses per command. */
    private class ReadinessPort(
        private val paramValues: Map<Int, String>,   // paramIndex → value string
        private val cmd74Fields: String = "0\t1\t1\t0\t2",  // rtcSynced=1, paperOk=1, receiptOpen=0, currencyMode=2
        private val cmd68Fields: String = "0\t60"            // reportsLeft=60
    ) : DatecsFiscalPort {
        override fun reachable() = true
        override fun execute(command: Int, payload: ByteArray): DatecsResponse {
            val data = when (command) {
                255 -> {
                    val idx = String(payload, Charsets.UTF_8).trim().toIntOrNull() ?: 0
                    val value = paramValues[idx] ?: "0"
                    "0\t$value".toByteArray(Charsets.UTF_8)
                }
                74 -> cmd74Fields.toByteArray(Charsets.UTF_8)
                68 -> cmd68Fields.toByteArray(Charsets.UTF_8)
                else -> "0\t".toByteArray(Charsets.UTF_8)
            }
            return DatecsResponse(0x20, command, data, byteArrayOf())
        }
    }

    private fun portReady() = ReadinessPort(
        paramValues = mapOf(
            0 to "1",  // fiscalized
            1 to "1",  // napRegistered
            2 to "0",  // zNeeded=false
            3 to "0",  // unsentWarning=false
            4 to "0"   // napBlockAt=none
        )
    )

    // ─── VG-10.1 ReadinessMatrix ─────────────────────────────────────────────
    // One row per rule: each field from the readiness response is correctly parsed.

    @Test fun ReadinessMatrix_fiscalizedParsed() {
        val result = DatecsReadiness.read(portReady())
        assertTrue("fiscalized must be true when cmd255[0]=1", result.fiscalized)
    }

    @Test fun ReadinessMatrix_notFiscalizedParsed() {
        val port = ReadinessPort(mapOf(0 to "0", 1 to "0", 2 to "0", 3 to "0", 4 to "0"))
        val result = DatecsReadiness.read(port)
        assertFalse("fiscalized must be false when cmd255[0]=0", result.fiscalized)
    }

    @Test fun ReadinessMatrix_zNeededParsed() {
        val port = ReadinessPort(mapOf(0 to "1", 1 to "1", 2 to "1", 3 to "0", 4 to "0"))
        val result = DatecsReadiness.read(port)
        assertTrue("zNeeded must be true when cmd255[2]=1", result.zNeeded)
    }

    @Test fun ReadinessMatrix_reportsLeftFromCmd68() {
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0", 4 to "0"),
            cmd68Fields = "0\t42"
        )
        val result = DatecsReadiness.read(port)
        assertEquals("reportsLeft must come from cmd68 response", 42, result.reportsLeft)
    }

    @Test fun ReadinessMatrix_rtcSyncedFromCmd74() {
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0", 4 to "0"),
            cmd74Fields = "0\t1\t1\t0\t2"
        )
        val result = DatecsReadiness.read(port)
        assertTrue("rtcSynced must be true when cmd74[1]=1", result.rtcSynced)
    }

    @Test fun ReadinessMatrix_paperNotOk() {
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0", 4 to "0"),
            cmd74Fields = "0\t1\t0\t0\t2"  // paperOk=0
        )
        val result = DatecsReadiness.read(port)
        assertFalse("paperOk must be false when cmd74[2]=0", result.paperOk)
    }

    @Test fun ReadinessMatrix_currencyModeFromCmd74() {
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0", 4 to "0"),
            cmd74Fields = "0\t1\t1\t0\t3"  // currencyMode=3
        )
        val result = DatecsReadiness.read(port)
        assertEquals("currencyMode must come from cmd74[4]", 3, result.currencyMode)
    }

    @Test fun ReadinessMatrix_napBlockAtParsed() {
        // Provide napBlockAt as epoch seconds for a specific instant.
        val blockInstant = ZonedDateTime.of(2025, 6, 1, 12, 0, 0, 0, SOFIA)
        val epochStr = blockInstant.toEpochSecond().toString()
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0", 4 to epochStr)
        )
        val result = DatecsReadiness.read(port)
        assertNotNull("napBlockAt must be non-null when epoch is set", result.napBlockAt)
        assertEquals("napBlockAt must match the epoch", blockInstant.toEpochSecond(),
            result.napBlockAt!!.toEpochSecond())
    }

    @Test fun ReadinessMatrix_napBlockAtNullWhenZero() {
        val result = DatecsReadiness.read(portReady())
        assertNull("napBlockAt must be null when cmd255[4]=0", result.napBlockAt)
    }

    @Test fun ReadinessMatrix_receiptOpenFromCmd74() {
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0", 4 to "0"),
            cmd74Fields = "0\t1\t1\t1\t2"  // receiptOpen=1
        )
        val result = DatecsReadiness.read(port)
        assertTrue("receiptOpen must be true when cmd74[3]=1", result.receiptOpen)
    }

    // ─── VG-10.2 BlockedByNapIsStructuredError ────────────────────────────────
    // When NAP block is active or imminent, evaluate() returns structured violations.

    @Test fun BlockedByNapIsStructuredError_activeBlock() {
        // napBlockAt is in the past → already blocked.
        val now = ZonedDateTime.now(SOFIA)
        val blockInstant = now.minusHours(1)
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0",
                4 to blockInstant.toEpochSecond().toString())
        )
        val result = DatecsReadiness.read(port, now)
        val violations = DatecsReadiness.evaluate(result, now)
        val codes = violations.map { it.code }
        assertTrue("active NAP block must produce ERR_BLOCKED_24H violation",
            DatecsReadiness.ERR_BLOCKED_24H in codes)
    }

    @Test fun BlockedByNapIsStructuredError_blockInLessThan72h() {
        val now = ZonedDateTime.now(SOFIA)
        val blockInstant = now.plusHours(48) // 48h from now = within 72h warn window
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0",
                4 to blockInstant.toEpochSecond().toString())
        )
        val result = DatecsReadiness.read(port, now)
        val violations = DatecsReadiness.evaluate(result, now)
        val codes = violations.map { it.code }
        assertTrue("NAP block within 72h must produce ERR_NAP_BLOCKED_72H violation",
            DatecsReadiness.ERR_NAP_BLOCKED_72H in codes)
    }

    @Test fun BlockedByNapIsStructuredError_zNeededIsViolation() {
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "1", 3 to "0", 4 to "0")  // zNeeded=1
        )
        val result = DatecsReadiness.read(port)
        val violations = DatecsReadiness.evaluate(result)
        val codes = violations.map { it.code }
        assertTrue("zNeeded=true must produce ERR_NEED_Z_REPORT violation",
            DatecsReadiness.ERR_NEED_Z_REPORT in codes)
    }

    @Test fun BlockedByNapIsStructuredError_noViolationsWhenReady() {
        val result = DatecsReadiness.read(portReady())
        val violations = DatecsReadiness.evaluate(result)
        assertTrue("ready device must have no violations", violations.isEmpty())
    }

    @Test fun BlockedByNapIsStructuredError_reportsLeftZeroIsViolation() {
        val port = ReadinessPort(
            mapOf(0 to "1", 1 to "1", 2 to "0", 3 to "0", 4 to "0"),
            cmd68Fields = "0\t0"  // reportsLeft=0
        )
        val result = DatecsReadiness.read(port)
        val violations = DatecsReadiness.evaluate(result)
        val codes = violations.map { it.code }
        assertTrue("reportsLeft=0 must produce ERR_MAX_ZERRORS violation",
            DatecsReadiness.ERR_MAX_ZERRORS in codes)
    }
}
