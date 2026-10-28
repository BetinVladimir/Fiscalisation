package com.beeloy.fiscal.bluecash
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
// TST-VENDOR-06: SEQ persistence + same-SEQ-on-restart = duplicate response, no re-execution.
private var failures = 0
private fun test(name: String, f: () -> Unit) {
    try { f() } catch (e: Throwable) { failures++; System.err.println("FAIL $name: ${e.message}") }
}
/** Build a valid Datecs answer frame (empty data) for given SEQ and CMD. */
private fun answerFrame(seq: Int, cmd: Int): ByteArray {
    val out = ArrayList<Byte>()
    fun word(v: Int) { for (sh in intArrayOf(12,8,4,0)) out += (0x30 + ((v shr sh) and 15)).toByte() }
    out += 1.toByte(); word(0x33); out += seq.toByte(); word(cmd)
    out += 4.toByte(); repeat(8) { out += 0x80.toByte() }; out += 5.toByte()
    val bcc = (1 until out.size).sumOf { out[it].toInt() and 0xFF } and 0xFFFF
    word(bcc); out += 3.toByte()
    return out.toByteArray()
}
fun main() {
    // Test 1: MemorySequenceStore persists SEQ correctly
    test("store_save_and_load") {
        val store = MemorySequenceStore()
        check(store.load() == null) { "initial load must be null" }
        store.save(0x42)
        check(store.load() == 0x42) { "must load saved seq=0x42" }
        store.save(0x7F)
        check(store.load() == 0x7F) { "must load overwritten seq=0x7F" }
    }
    // Test 2: Protocol with stored SEQ uses that SEQ (not default 0x20)
    test("protocol_uses_stored_seq_not_default") {
        val store = MemorySequenceStore(); store.save(0x35)
        val resp = answerFrame(0x35, 74)
        val out = ByteArrayOutputStream()
        val proto = DatecsSocketProtocol(ByteArrayInputStream(resp), out, store)
        check(!proto.seqUnknown) { "seqUnknown must be false when store has value" }
        check(proto.sequence == 0x35) { "must use stored seq=0x35, got ${proto.sequence}" }
        val result = proto.execute(74)
        check(result.sequence == 0x35) { "response seq must be 0x35" }
        // After execute, store holds next seq = 0x36
        check(store.load() == 0x36) { "store must advance to 0x36, got ${store.load()}" }
    }
    // Test 3: Restart after crash (store not updated) → same SEQ sent again
    // "FU caches response by SEQ; same SEQ → returns cached response, doc counter = 1"
    test("crash_before_store_save_same_seq_on_retry") {
        // Pre-crash: store had SEQ=0x2A (the value the NEXT execute would use)
        val storeFirst = MemorySequenceStore(); storeFirst.save(0x2A)
        val resp1 = answerFrame(0x2A, 56)
        val out1 = ByteArrayOutputStream()
        val proto1 = DatecsSocketProtocol(ByteArrayInputStream(resp1), out1, storeFirst)
        proto1.execute(56)  // normal execute; store now holds 0x2B
        val req1 = out1.toByteArray()

        // Simulate restart: crash prevented store from advancing; store still has 0x2A
        val storeRetry = MemorySequenceStore(); storeRetry.save(0x2A)
        val resp2 = answerFrame(0x2A, 56)  // device returns cached response for SEQ=0x2A
        val out2 = ByteArrayOutputStream()
        val proto2 = DatecsSocketProtocol(ByteArrayInputStream(resp2), out2, storeRetry)
        proto2.execute(56)
        val req2 = out2.toByteArray()

        // Both requests must use the same SEQ byte (position 5 in the Datecs frame)
        check(req1.size == req2.size) { "request frames must be same size" }
        check(req1[5] == req2[5]) {
            "retry must use same SEQ: first=${req1[5].toInt() and 0xFF}, retry=${req2[5].toInt() and 0xFF}"
        }
        check((req1[5].toInt() and 0xFF) == 0x2A) { "SEQ must be 0x2A, got ${req1[5].toInt() and 0xFF}" }
    }
    // Test 4: SEQ rollover from 0x7F to 0x20
    test("seq_rollover_0x7f_to_0x20") {
        val store = MemorySequenceStore(); store.save(0x7F)
        val resp = answerFrame(0x7F, 74)
        val out = ByteArrayOutputStream()
        val proto = DatecsSocketProtocol(ByteArrayInputStream(resp), out, store)
        proto.execute(74)
        check(store.load() == 0x20) { "SEQ must wrap 0x7F→0x20, got ${store.load()}" }
    }
    // Test 5: No store (null) → default SEQ=0x20 and seqUnknown=true
    test("no_store_default_seq_unknown") {
        val proto = DatecsSocketProtocol(ByteArrayInputStream(ByteArray(0)), ByteArrayOutputStream())
        check(proto.seqUnknown) { "seqUnknown must be true with no store" }
        check(proto.sequence == 0x20) { "default seq must be 0x20, got ${proto.sequence}" }
    }
    check(failures == 0) { "$failures SEQ caching vendor test failures" }
    println("actual Kotlin SEQ caching vendor tests: OK")
}
