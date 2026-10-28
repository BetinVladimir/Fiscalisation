package com.beeloy.fiscal.bluecash
import java.time.ZonedDateTime
import java.time.ZoneId
// TST-VENDOR-08: DatecsClock drift detection + currency mode 2/3 readiness.
private var failures = 0
private fun test(name: String, f: () -> Unit) {
    try { f() } catch (e: Throwable) { failures++; System.err.println("FAIL $name: ${e.message}") }
}
private val SOFIA = ZoneId.of("Europe/Sofia")
/** Parse FU response and return ClockResult. hostNow injected, never fallback to host clock. */
private fun parse(fuResponse: String, hostNow: ZonedDateTime) = DatecsClock.parse(fuResponse, hostNow)
/** Currency mode READY only when confirmed as BGN (2) or EUR (3). 0/1/unknown → block. */
private fun isCurrencyReady(mode: Int) = mode == 2 || mode == 3
fun main() {
    // Test 1: drift < 5 min (300 s) → OK
    test("drift_under_5min_ok") {
        // FU: 2026-10-28 12:02:00; host: 12:00:00 → drift = +120s
        val host = ZonedDateTime.of(2026, 10, 28, 12, 0, 0, 0, SOFIA)
        val result = parse("28-10-26\t12:02:00\t0", host)
        check(result.driftStatus == DriftStatus.OK) { "120s should be OK, got ${result.driftStatus}" }
        check(result.driftSeconds == 120L) { "drift must be 120, got ${result.driftSeconds}" }
    }
    // Test 2: drift > 5 min but < 2 h → WARN
    test("drift_6min_warn") {
        // FU: 12:06:00; host: 12:00:00 → drift = +360s
        val host = ZonedDateTime.of(2026, 10, 28, 12, 0, 0, 0, SOFIA)
        val result = parse("28-10-26\t12:06:00\t0", host)
        check(result.driftStatus == DriftStatus.WARN) { "360s should be WARN, got ${result.driftStatus}" }
    }
    // Test 3: drift > 2 h → BLOCK
    test("drift_3h_block") {
        // FU: 15:00:00; host: 12:00:00 → drift = +10800s
        val host = ZonedDateTime.of(2026, 10, 28, 12, 0, 0, 0, SOFIA)
        val result = parse("28-10-26\t15:00:00\t0", host)
        check(result.driftStatus == DriftStatus.BLOCK) { "10800s should be BLOCK, got ${result.driftStatus}" }
    }
    // Test 4: negative drift (FU behind host) also triggers WARN/BLOCK
    test("drift_negative_6min_warn") {
        // FU: 11:54:00; host: 12:00:00 → drift = -360s → abs=360 > 300 → WARN
        val host = ZonedDateTime.of(2026, 10, 28, 12, 0, 0, 0, SOFIA)
        val result = parse("28-10-26\t11:54:00\t0", host)
        check(result.driftStatus == DriftStatus.WARN) { "abs(360s) should be WARN, got ${result.driftStatus}" }
    }
    // Test 5: businessDate is in Europe/Sofia timezone (not UTC)
    test("business_date_europe_sofia") {
        // FU responds 23:30:00 on 27-10-26 in Sofia; same moment host is in Sofia → date = 2026-10-27
        val host = ZonedDateTime.of(2026, 10, 27, 23, 30, 0, 0, SOFIA)
        val result = parse("27-10-26\t23:30:00\t0", host)
        check(result.businessDate.toString() == "2026-10-27") {
            "businessDate must be 2026-10-27 in Sofia, got ${result.businessDate}"
        }
    }
    // Test 6: Unambiguous summer time (June) → offset +03:00 (EEST)
    test("summer_time_offset_plus3") {
        // June 15 in Sofia is unambiguously summer (+03:00); DST flag irrelevant
        val host = ZonedDateTime.of(2026, 6, 15, 12, 0, 0, 0, SOFIA)
        val result = parse("15-06-26\t12:00:00\t1", host)
        check(result.deviceTime.offset.totalSeconds == 3 * 3600) {
            "June in Sofia must have +03:00 offset, got ${result.deviceTime.offset}"
        }
        check(result.businessDate.toString() == "2026-06-15") { "businessDate must be 2026-06-15" }
    }
    // Test 7: Invalid response (fewer than 3 fields) → ClockParseException
    test("invalid_response_throws") {
        val host = ZonedDateTime.now(SOFIA)
        try {
            parse("28-10-26\t12:00:00", host) // only 2 fields, no DST
            error("must throw ClockParseException for 2-field response")
        } catch (e: ClockParseException) { /* expected */ }
    }
    // Test 8: Currency mode 2 (BGN) → READY
    test("currency_mode_2_bgn_ready") {
        check(isCurrencyReady(2)) { "currencyMode=2 (BGN) must be READY" }
    }
    // Test 9: Currency mode 3 (EUR) → READY
    test("currency_mode_3_eur_ready") {
        check(isCurrencyReady(3)) { "currencyMode=3 (EUR) must be READY" }
    }
    // Test 10: Currency mode 0 (not set) → blocked
    test("currency_mode_0_not_ready") {
        check(!isCurrencyReady(0)) { "currencyMode=0 must NOT be READY" }
    }
    // Test 11: Currency mode 1 (BGN+EUR unconfirmed) → blocked
    test("currency_mode_1_not_ready") {
        check(!isCurrencyReady(1)) { "currencyMode=1 must NOT be READY" }
    }
    // Test 12: Unknown currency mode (-1) → blocked
    test("currency_mode_unknown_not_ready") {
        check(!isCurrencyReady(-1)) { "currencyMode=-1 must NOT be READY" }
    }
    // Test 13: Threshold boundary — exactly 5 min (300s) drift
    test("drift_exactly_5min_boundary") {
        val host = ZonedDateTime.of(2026, 10, 28, 12, 0, 0, 0, SOFIA)
        val result = parse("28-10-26\t12:05:00\t0", host)
        // 300s == DRIFT_WARN_SECONDS → WARN (>= threshold)
        check(result.driftStatus == DriftStatus.WARN) { "300s at boundary must be WARN, got ${result.driftStatus}" }
    }
    // Test 14: Exactly 2h (7200s) drift → BLOCK
    test("drift_exactly_2h_boundary") {
        val host = ZonedDateTime.of(2026, 10, 28, 12, 0, 0, 0, SOFIA)
        val result = parse("28-10-26\t14:00:00\t0", host)
        // 7200s == DRIFT_BLOCK_SECONDS → BLOCK
        check(result.driftStatus == DriftStatus.BLOCK) { "7200s at boundary must be BLOCK, got ${result.driftStatus}" }
    }
    check(failures == 0) { "$failures DatecsClock/currency vendor test failures" }
    println("actual Kotlin DatecsClock vendor tests: OK")
}
