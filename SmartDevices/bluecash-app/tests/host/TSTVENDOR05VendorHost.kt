package com.beeloy.fiscal.bluecash
// TST-VENDOR-05: DatecsStatus bit-map + DatecsErrorCatalog category completeness.
private var failures = 0
private fun test(name: String, f: () -> Unit) {
    try { f() } catch (e: Throwable) { failures++; System.err.println("FAIL $name: ${e.message}") }
}
fun main() {
    // All 8 status bytes: bit5 (0x20) = error flag → fatal
    for (i in 0..7) {
        test("bit5_byte${i}_fatal") {
            val raw = ByteArray(8); raw[i] = 0x20.toByte()
            check(DatecsStatus.parse(raw).fatal) { "bit5 byte$i must be fatal" }
            check(!DatecsStatus.parse(raw).warning || i == 0) // byte0 may also trigger warning if other bits set; only testing fatal here
        }
    }
    // Byte0 bit1 (0x02) = near-paper-end → warning, NOT fatal
    test("byte0_bit1_near_paper_end_warning") {
        val raw = ByteArray(8); raw[0] = 0x02.toByte()
        val s = DatecsStatus.parse(raw)
        check(!s.fatal) { "bit1 byte0 must not be fatal" }
        check(s.warning) { "bit1 byte0 must be warning (near-paper-end)" }
    }
    // Byte0 bit4 (0x10) = FM-full → warning, NOT fatal
    test("byte0_bit4_fm_full_warning") {
        val raw = ByteArray(8); raw[0] = 0x10.toByte()
        val s = DatecsStatus.parse(raw)
        check(!s.fatal) { "bit4 byte0 must not be fatal" }
        check(s.warning) { "bit4 byte0 must be warning (fm-full)" }
    }
    // All zeros → OK (normal)
    test("all_zeros_ok") {
        val s = DatecsStatus.parse(ByteArray(8))
        check(!s.fatal) { "all-zero must not be fatal" }
        check(!s.warning) { "all-zero must not be warning" }
    }
    // Byte0 bit5+bit1 → both fatal AND warning
    test("byte0_bit5_and_bit1_fatal_and_warning") {
        val raw = ByteArray(8); raw[0] = (0x20 or 0x02).toByte()
        val s = DatecsStatus.parse(raw)
        check(s.fatal) { "bit5+bit1 must be fatal" }
        check(s.warning) { "bit5+bit1 must be warning" }
    }
    // Empty response: request frame (no 04..05 STAT section) → decode fails
    test("request_frame_decode_rejected") {
        val reqFrame = DatecsFrameCodec.encode(0x24, 33, byteArrayOf())
        // Request frame is < 25 bytes → decode must throw
        try {
            DatecsFrameCodec.decode(reqFrame)
            error("request frame must not decode as answer")
        } catch (e: IllegalArgumentException) { /* expected: DATECS_BAD_FRAME */ }
    }
    // DatecsErrorCatalog: unknown code → null (UNKNOWN_DEVICE_ERROR, not success)
    test("unknown_error_code_null") {
        check(DatecsErrorCatalog.lookup(-999999) == null) { "unknown code must return null" }
    }
    // DatecsErrorCatalog: known GENERAL code → correct category
    test("known_error_code_general") {
        val e = DatecsErrorCatalog.lookup(-100000) ?: error("entry -100000 not found")
        check(e.category == DatecsErrorCatalog.ErrorCategory.GENERAL) { "wrong category: ${e.category}" }
    }
    // DatecsErrorCatalog: PAPER category
    test("known_error_code_paper") {
        val e = DatecsErrorCatalog.lookup(-100041) ?: error("entry -100041 not found")
        check(e.category == DatecsErrorCatalog.ErrorCategory.PAPER) { "wrong category: ${e.category}" }
    }
    // DatecsErrorCatalog: FISCAL category
    test("known_error_code_fiscal") {
        val e = DatecsErrorCatalog.lookup(-100010) ?: error("entry -100010 not found")
        check(e.category == DatecsErrorCatalog.ErrorCategory.FISCAL) { "wrong category: ${e.category}" }
    }
    // All four categories present in the catalog
    test("all_categories_present") {
        val cats = DatecsErrorCatalog.entries.map { it.category }.toSet()
        for (cat in DatecsErrorCatalog.ErrorCategory.values()) {
            check(cat in cats) { "category $cat missing from catalog" }
        }
    }
    // Minimum 13 entries in catalog (Datecs §7)
    test("catalog_minimum_13_entries") {
        check(DatecsErrorCatalog.entries.size >= 13) {
            "need >= 13 entries, got ${DatecsErrorCatalog.entries.size}"
        }
    }
    // DatecsStatus requires exactly 8 bytes
    test("status_wrong_size_throws") {
        try { DatecsStatus.parse(ByteArray(7)); error("should throw for 7 bytes") }
        catch (e: IllegalArgumentException) { /* expected */ }
    }
    check(failures == 0) { "$failures DatecsStatus/ErrorCatalog vendor test failures" }
    println("actual Kotlin DatecsStatus vendor tests: OK")
}
