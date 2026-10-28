package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

/**
 * TG-006.1–2: Typed response parsers for Datecs commands.
 *
 * One test per command (48, 49, 53, 56, 43, 69/70, 38, 39, 42) + broken-response tests.
 * Invariant 10: parsers never throw externally — broken inputs must return null, not throw.
 * Invariant 6: cmd56 returns null when fiscal reference is missing.
 *
 * Response data is constructed inline without going through the frame codec to isolate
 * response parsing from frame transport concerns.
 */
class DatecsResponsesTest {

    private fun response(cmd: Int, data: String) =
        DatecsResponse(
            sequence = 0x20,
            command = cmd,
            data = DatecsText.encode(data),
            status = ByteArray(8)
        )

    private fun responseRaw(cmd: Int, data: ByteArray) =
        DatecsResponse(sequence = 0x20, command = cmd, data = data, status = ByteArray(8))

    // ── Cmd 48: Open fiscal receipt ──────────────────────────────────────────

    @Test fun cmd48OkResponseParsed() {
        val resp = response(48, "0\t")
        val parsed = resp.parseCmd48()
        assertNotNull("cmd48: ok response must parse to non-null", parsed)
        assertEquals("cmd48: errorCode must be 0", 0, parsed!!.errorCode)
    }

    @Test fun cmd48ErrorResponseParsed() {
        val resp = response(48, "1\t")
        val parsed = resp.parseCmd48()
        assertNotNull("cmd48: error response must parse (error code preserved)", parsed)
        assertEquals("cmd48: non-zero error code preserved", 1, parsed!!.errorCode)
    }

    @Test fun cmd48BrokenResponseReturnsNull() {
        val resp = responseRaw(48, byteArrayOf())
        val parsed = resp.parseCmd48()
        assertNull("cmd48: empty data must return null (invariant 10)", parsed)
    }

    @Test fun cmd48GarbageDataReturnsNull() {
        val resp = responseRaw(48, byteArrayOf(0x01, 0x7F, 0x42))
        val parsed = resp.parseCmd48()
        // Non-UTF data that doesn't parse as an integer must return null
        assertNull("cmd48: non-integer data must return null (invariant 10)", parsed)
    }

    // ── Cmd 49: Add fiscal line ───────────────────────────────────────────────

    @Test fun cmd49OkResponseParsed() {
        val resp = response(49, "0\t")
        val parsed = resp.parseCmd49()
        assertNotNull("cmd49: ok response must parse", parsed)
        assertEquals("cmd49: errorCode 0", 0, parsed!!.errorCode)
    }

    @Test fun cmd49BrokenResponseReturnsNull() {
        val resp = responseRaw(49, byteArrayOf())
        assertNull("cmd49: empty data must return null", resp.parseCmd49())
    }

    // ── Cmd 53: Payment ───────────────────────────────────────────────────────

    @Test fun cmd53OkResponseParsed() {
        val resp = response(53, "0\t")
        val parsed = resp.parseCmd53()
        assertNotNull("cmd53: ok response must parse", parsed)
        assertEquals("cmd53: errorCode 0", 0, parsed!!.errorCode)
    }

    @Test fun cmd53BrokenResponseReturnsNull() {
        val resp = responseRaw(53, byteArrayOf())
        assertNull("cmd53: empty data must return null", resp.parseCmd53())
    }

    // ── Cmd 56: Close fiscal receipt (most important — carries fiscal ref) ───

    @Test fun cmd56OkResponseYieldsFiscalRef() {
        // Response: "0\tBON-001\t20240101\t123456"
        val resp = response(56, "0\tBON-001\t20240101\t123456")
        val parsed = resp.parseCmd56()
        assertNotNull("cmd56: ok response must parse to non-null", parsed)
        assertEquals("cmd56: fiscal reference from field[1]", "BON-001", parsed!!.fiscalRef)
    }

    @Test fun cmd56ErrorCodeReturnsNull() {
        // Non-zero error code → receipt not closed → null (invariant 6: no fake ref)
        val resp = response(56, "1\tBON-999\t")
        val parsed = resp.parseCmd56()
        assertNull("cmd56: non-zero error must return null (invariant 6)", parsed)
    }

    @Test fun cmd56MissingRefReturnsNull() {
        // errorCode=0 but no fiscal reference → null (invariant 6)
        val resp = response(56, "0\t\t")
        val parsed = resp.parseCmd56()
        assertNull("cmd56: blank fiscal ref must return null (invariant 6)", parsed)
    }

    @Test fun cmd56OnlyErrorCodeReturnsNull() {
        val resp = response(56, "0\t")
        // Field[1] is empty string → null
        val parsed = resp.parseCmd56()
        assertNull("cmd56: missing field[1] must return null (invariant 6)", parsed)
    }

    @Test fun cmd56BrokenDataReturnsNull() {
        val resp = responseRaw(56, byteArrayOf())
        assertNull("cmd56: empty data must return null (invariant 10)", resp.parseCmd56())
    }

    // ── Cmd 43: Storno open (BC-50 storno cmd43) ──────────────────────────────

    @Test fun cmd43OkResponseParsed() {
        val resp = response(43, "0\t")
        val parsed = resp.parseCmd43()
        assertNotNull("cmd43: ok response must parse", parsed)
        assertEquals("cmd43: errorCode 0", 0, parsed!!.errorCode)
    }

    @Test fun cmd43ErrorResponseParsed() {
        val resp = response(43, "5\t")
        val parsed = resp.parseCmd43()
        assertNotNull("cmd43: error response must parse", parsed)
        assertEquals("cmd43: error code preserved", 5, parsed!!.errorCode)
    }

    @Test fun cmd43BrokenDataReturnsNull() {
        val resp = responseRaw(43, byteArrayOf())
        assertNull("cmd43: empty data must return null (invariant 10)", resp.parseCmd43())
    }

    // ── Cmd 69/70: X/Z daily report ──────────────────────────────────────────

    @Test fun cmd69OkResponseParsed() {
        // Response: "0\t5\t100.00\t50.00\t20.00\t0.00"
        val resp = response(69, "0\t5\t100.00\t50.00\t20.00\t0.00")
        val parsed = resp.parseCmd69()
        assertNotNull("cmd69: ok response must parse", parsed)
        assertEquals("cmd69: reportCount", "5", parsed!!.reportCount)
        assertEquals("cmd69: totalA", "100.00", parsed.totalA)
        assertEquals("cmd69: totalB", "50.00", parsed.totalB)
    }

    @Test fun cmd69ErrorCodeReturnsNull() {
        val resp = response(69, "1\t5\t100.00\t")
        assertNull("cmd69: non-zero error must return null", resp.parseCmd69())
    }

    @Test fun cmd69MissingReportCountReturnsNull() {
        val resp = response(69, "0\t")
        // field[1] is empty string — returns null
        val parsed = resp.parseCmd69()
        assertNull("cmd69: missing reportCount field must return null", parsed)
    }

    @Test fun cmd69BrokenDataReturnsNull() {
        val resp = responseRaw(69, byteArrayOf())
        assertNull("cmd69: empty data must return null (invariant 10)", resp.parseCmd69())
    }

    @Test fun cmd70OkResponseParsed() {
        // Cmd70 uses existing parseCashInOutResponse — verify it still works
        val resp = response(70, "0\t150.00\t50.00\t100.00\t")
        val parsed = DatecsPayloads.parseCashInOutResponse(resp)
        assertNotNull("cmd70: ok response must parse", parsed)
        assertEquals("cmd70: cashIn",      java.math.BigDecimal("150.00"), parsed!!.cashIn)
        assertEquals("cmd70: cashOut",     java.math.BigDecimal("50.00"),  parsed.cashOut)
        assertEquals("cmd70: cashBalance", java.math.BigDecimal("100.00"), parsed.cashBalance)
    }

    @Test fun cmd70BrokenDataReturnsNull() {
        val resp = responseRaw(70, byteArrayOf())
        val parsed = DatecsPayloads.parseCashInOutResponse(resp)
        assertNull("cmd70: empty data must return null (invariant 10)", parsed)
    }

    // ── Cmd 38: Print fiscal sub-total ────────────────────────────────────────

    @Test fun cmd38OkResponseParsed() {
        val resp = response(38, "0\t")
        val parsed = resp.parseCmd38()
        assertNotNull("cmd38: ok response must parse", parsed)
        assertEquals("cmd38: errorCode 0", 0, parsed!!.errorCode)
    }

    @Test fun cmd38BrokenDataReturnsNull() {
        val resp = responseRaw(38, byteArrayOf())
        assertNull("cmd38: empty data must return null (invariant 10)", resp.parseCmd38())
    }

    // ── Cmd 39: Print receipt total ───────────────────────────────────────────

    @Test fun cmd39OkResponseParsed() {
        val resp = response(39, "0\t")
        val parsed = resp.parseCmd39()
        assertNotNull("cmd39: ok response must parse", parsed)
        assertEquals("cmd39: errorCode 0", 0, parsed!!.errorCode)
    }

    @Test fun cmd39BrokenDataReturnsNull() {
        val resp = responseRaw(39, byteArrayOf())
        assertNull("cmd39: empty data must return null (invariant 10)", resp.parseCmd39())
    }

    // ── Cmd 42: Print department sub-total ───────────────────────────────────

    @Test fun cmd42OkResponseParsed() {
        val resp = response(42, "0\t")
        val parsed = resp.parseCmd42()
        assertNotNull("cmd42: ok response must parse", parsed)
        assertEquals("cmd42: errorCode 0", 0, parsed!!.errorCode)
    }

    @Test fun cmd42BrokenDataReturnsNull() {
        val resp = responseRaw(42, byteArrayOf())
        assertNull("cmd42: empty data must return null (invariant 10)", resp.parseCmd42())
    }

    // ── requireOk() integration ───────────────────────────────────────────────

    @Test fun requireOkPassesOnZeroErrorCode() {
        val resp = response(56, "0\tBON-555\t")
        // requireOk must not throw when field[0] == "0"
        assertSame("requireOk returns self", resp, resp.requireOk())
    }

    @Test fun requireOkThrowsOnNonZeroErrorCode() {
        val resp = response(56, "3\t")
        try {
            resp.requireOk()
            fail("requireOk must throw DatecsFailure.Rejected on non-zero error code")
        } catch (e: DatecsFailure.Rejected) {
            assertEquals("Rejected errorCode", 3, e.keyCode)
        }
    }

    @Test fun requireOkThrowsOnEmptyData() {
        val resp = responseRaw(56, byteArrayOf())
        try {
            resp.requireOk()
            fail("requireOk must throw on empty data")
        } catch (e: DatecsFailure.Rejected) {
            // expected
        }
    }

    // ── Cmd 56: Cyrillic reference (CP1251, invariant 7) ─────────────────────

    @Test fun cmd56CyrillicFiscalRefRoundTrip() {
        // Fiscal reference can contain Cyrillic characters in CP1251
        val ref = "БОН-001"
        val data = DatecsText.encode("0\t$ref\t")
        val resp = responseRaw(56, data)
        val parsed = resp.parseCmd56()
        assertNotNull("cmd56 Cyrillic ref must parse", parsed)
        assertEquals("cmd56: Cyrillic fiscal reference round-trip", ref, parsed!!.fiscalRef)
    }
}
