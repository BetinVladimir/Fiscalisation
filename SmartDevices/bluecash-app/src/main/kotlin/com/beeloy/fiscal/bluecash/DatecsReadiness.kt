package com.beeloy.fiscal.bluecash

import java.time.Duration
import java.time.ZonedDateTime

/**
 * VG-10: Datecs FU readiness state, assembled from cmd 255 (×N), cmd 74, cmd 68.
 *
 * Fields mirror the NAP parameter names from the Datecs PM.
 *
 * @param fiscalized         FU is fiscalised (has a fiscal memory number).
 * @param napRegistered      FU is registered with the NAP (tax authority).
 * @param napBlockAt         Time after which NAP will block this FU, or null if not threatened.
 * @param unsentWarning      True when unsent documents are approaching the NAP block threshold.
 * @param zNeeded            True when a Z-report is required before further sales.
 * @param reportsLeft        Number of remaining Z-reports available in the fiscal memory (cmd 68).
 * @param rtcSynced          True when the FU real-time clock is synchronised.
 * @param paperOk            True when the receipt paper is present and not near end.
 * @param receiptOpen        True when a fiscal receipt is currently open.
 * @param currencyMode       Currency mode index (2=BGN, 3=EUR as per BC-50 profile).
 */
data class ReadinessResult(
    val fiscalized: Boolean,
    val napRegistered: Boolean,
    val napBlockAt: ZonedDateTime?,
    val unsentWarning: Boolean,
    val zNeeded: Boolean,
    val reportsLeft: Int,
    val rtcSynced: Boolean,
    val paperOk: Boolean,
    val receiptOpen: Boolean,
    val currencyMode: Int
)

/**
 * Readiness rule violations detected from a [ReadinessResult].
 *
 * Each entry is an error code string such as "ERR_NAP_BLOCKED_72H".
 */
data class ReadinessViolation(val code: String, val detail: String = "")

object DatecsReadiness {

    // NAP error codes as per Datecs PM error table.
    const val ERR_NAP_BLOCKED_72H          = "ERR_NAP_BLOCKED_72H"          // -110210
    const val ERR_BLOCKED_MAX_SELLERRORS   = "ERR_BLOCKED_MAX_SELLERRORS"   // -110225
    const val ERR_BLOCKED_24H              = "ERR_BLOCKED_24H"              // -110226
    const val ERR_MAX_ZERRORS              = "ERR_MAX_ZERRORS"              // -110217
    const val ERR_END_OF_24_HOUR_PERIOD    = "ERR_END_OF_24_HOUR_PERIOD"    // -111024
    const val ERR_NEED_Z_REPORT            = "ERR_NEED_Z_REPORT"            // -104000
    const val ERR_WAIT_UNTIL_MIDNIGHT      = "ERR_WAIT_UNTIL_MIDNIGHT"      // -111098

    // Warning: NAP block approaching within this window.
    const val NAP_WARN_HOURS = 24L
    // Block sales when NAP block is within this window.
    const val NAP_BLOCK_HOURS = 72L

    /**
     * VG-10.1: Read readiness from the fiscal device.
     *
     * Sends cmd 255 (×N for each NAP parameter), cmd 74, and cmd 68 to the device.
     * Parsed results are assembled into a [ReadinessResult].
     *
     * In unit tests, supply a [DatecsFiscalPort] mock that returns structured
     * tab-separated responses:
     *   cmd255 response: `0\t<field_name>\t<value>` (one call per parameter)
     *   cmd74  response: `0\t<status_flags>`
     *   cmd68  response: `0\t<reports_left_int>`
     *
     * @param port     fiscal port to query
     * @param hostNow  current wall clock (injected for testability)
     * @return [ReadinessResult] assembled from device responses
     * @throws DatecsFailure if a command fails or cannot be parsed
     */
    fun read(port: DatecsFiscalPort, hostNow: ZonedDateTime = ZonedDateTime.now()): ReadinessResult {
        // cmd 255: read individual NAP parameters by name.
        // Parameter index mapping (Datecs PM §4.x):
        //   0 = Fiscalized, 1 = NapRegistered, 2 = DFR_needed (Z report), 3 = EcrUnsentWarning,
        //   4 = NapBlockDateTime (epoch seconds string, or "0")
        val fiscalized      = readBoolParam(port, 0)
        val napRegistered   = readBoolParam(port, 1)
        val zNeeded         = readBoolParam(port, 2)
        val unsentWarning   = readBoolParam(port, 3)
        val napBlockEpoch   = readStringParam(port, 4)

        val napBlockAt: ZonedDateTime? = napBlockEpoch
            .takeIf { it.isNotBlank() && it != "0" }
            ?.toLongOrNull()
            ?.let { java.time.Instant.ofEpochSecond(it).atZone(java.time.ZoneId.of("Europe/Sofia")) }

        // cmd 74: device status flags (paper, receipt open, RTC sync, currency mode).
        // Response: "0\t<rtcSynced:0/1>\t<paperOk:0/1>\t<receiptOpen:0/1>\t<currencyMode:int>"
        val cmd74Fields = parseResponse(port.execute(74))
        val rtcSynced    = cmd74Fields.getOrNull(1)?.trim() == "1"
        val paperOk      = cmd74Fields.getOrNull(2)?.trim() == "1"
        val receiptOpen  = cmd74Fields.getOrNull(3)?.trim() == "1"
        val currencyMode = cmd74Fields.getOrNull(4)?.trim()?.toIntOrNull() ?: 0

        // cmd 68: reports left in fiscal memory.
        // Response: "0\t<reportsLeft:int>"
        val cmd68Fields  = parseResponse(port.execute(68))
        val reportsLeft  = cmd68Fields.getOrNull(1)?.trim()?.toIntOrNull() ?: 0

        return ReadinessResult(
            fiscalized    = fiscalized,
            napRegistered = napRegistered,
            napBlockAt    = napBlockAt,
            unsentWarning = unsentWarning,
            zNeeded       = zNeeded,
            reportsLeft   = reportsLeft,
            rtcSynced     = rtcSynced,
            paperOk       = paperOk,
            receiptOpen   = receiptOpen,
            currencyMode  = currencyMode
        )
    }

    /**
     * VG-10.2: Evaluate readiness rules and return any violations.
     *
     * Returns an empty list when the device is ready to accept a sale.
     * Each entry is a structured error that the POS UI can display.
     */
    fun evaluate(result: ReadinessResult, hostNow: ZonedDateTime = ZonedDateTime.now()): List<ReadinessViolation> {
        val violations = mutableListOf<ReadinessViolation>()

        if (!result.fiscalized) {
            violations += ReadinessViolation(ERR_NAP_BLOCKED_72H, "FU not fiscalized")
        }
        if (result.napBlockAt != null) {
            val hoursUntilBlock = Duration.between(hostNow.toInstant(), result.napBlockAt.toInstant()).toHours()
            when {
                hoursUntilBlock <= 0 ->
                    violations += ReadinessViolation(ERR_BLOCKED_24H, "NAP block active")
                hoursUntilBlock <= NAP_BLOCK_HOURS ->
                    violations += ReadinessViolation(ERR_NAP_BLOCKED_72H,
                        "NAP block in ${hoursUntilBlock}h")
            }
        }
        if (result.unsentWarning) {
            violations += ReadinessViolation(ERR_BLOCKED_MAX_SELLERRORS, "Unsent documents near limit")
        }
        if (result.zNeeded) {
            violations += ReadinessViolation(ERR_NEED_Z_REPORT, "Z-report required before next sale")
        }
        if (result.reportsLeft == 0) {
            violations += ReadinessViolation(ERR_MAX_ZERRORS, "No Z-reports left in fiscal memory")
        }

        return violations
    }

    // ── private helpers ────────────────────────────────────────────────────────

    private fun parseResponse(response: DatecsResponse): List<String> =
        DatecsText.decode(response.data).split('\t')

    private fun readBoolParam(port: DatecsFiscalPort, paramIndex: Int): Boolean {
        // Send cmd 255 with the parameter index as the payload.
        val payload = DatecsText.encode("$paramIndex")
        val response = port.execute(255, payload)
        val fields = parseResponse(response)
        return fields.getOrNull(1)?.trim() == "1"
    }

    private fun readStringParam(port: DatecsFiscalPort, paramIndex: Int): String {
        val payload = DatecsText.encode("$paramIndex")
        val response = port.execute(255, payload)
        val fields = parseResponse(response)
        return fields.getOrNull(1)?.trim() ?: ""
    }
}
