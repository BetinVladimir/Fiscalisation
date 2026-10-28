package com.beeloy.fiscal.bluecash
import java.time.Instant
// TST-VENDOR-07: DatecsRecovery.resolve() — COMMITTED / NOT_COMMITTED / UNKNOWN scenarios.
private var failures = 0
private fun test(name: String, f: () -> Unit) {
    try { f() } catch (e: Throwable) { failures++; System.err.println("FAIL $name: ${e.message}") }
}
private val DUMMY_JOURNAL = object : TransactionJournal {
    override fun find(id: String) = emptyList<JournalRecord>()
    override fun append(id: String, type: String, payload: String) =
        JournalRecord(0, id, type, "2024-01-01T00:00:00Z", payload, null, "", "", false)
    override fun pending(limit: Int) = emptyList<JournalRecord>()
    override fun acknowledge(seq: Long, hash: String, ackId: String) {}
    override fun purgeBefore(cutoff: Instant) = 0
    override fun checkpoint() = Pair(0L, null as String?)
}
private fun mockResp(data: String) =
    DatecsResponse(0x20, 0, DatecsText.encode(data), ByteArray(8) { 0x80.toByte() })
/** Port that returns scripted responses in order, verifying command numbers. */
private class ScriptedPort(vararg scripts: Pair<Int, String>) : DatecsFiscalPort {
    private val q = ArrayDeque(scripts.toList())
    override fun execute(cmd: Int, payload: ByteArray): DatecsResponse {
        val (expectedCmd, data) = q.removeFirst()
        check(cmd == expectedCmd) { "ScriptedPort: expected cmd$expectedCmd, got cmd$cmd" }
        return mockResp(data)
    }
    override fun reachable() = true
}
// Pre-open snapshot: receipt counter at 10, UNP="BASE_UNP"
private val PRE = PreOpenSnapshot(isOpen = 0, receiptNumber = 10, unp = "BASE_UNP", lastFiscalDoc = 10)
fun main() {
    // COMMITTED: IsOpen=0, Number(11) > pre.receiptNumber(10), currentUnp matches expectedUnp
    test("resolve_committed") {
        val port = ScriptedPort(
            76 to "0\t0\t11",         // cmd76: IsOpen=0, Number=11
            255 to "0\tOP_UNP_42"     // cmd255(UNP): currentUnp="OP_UNP_42"
        )
        val state = DatecsRecovery(port, DUMMY_JOURNAL).resolve(PRE, "OP_UNP_42")
        check(state == ResolutionState.COMMITTED) { "expected COMMITTED, got $state" }
    }
    // NOT_COMMITTED: IsOpen=0, number not greater than pre, currentUnp == pre.unp
    test("resolve_not_committed_same_unp") {
        val port = ScriptedPort(
            76 to "0\t0\t10",         // cmd76: IsOpen=0, Number=10 (not > 10)
            255 to "0\tBASE_UNP"      // cmd255(UNP): matches pre.unp
        )
        val state = DatecsRecovery(port, DUMMY_JOURNAL).resolve(PRE, "OP_UNP_42")
        check(state == ResolutionState.NOT_COMMITTED) { "expected NOT_COMMITTED, got $state" }
    }
    // UNKNOWN: IsOpen=1 (receipt still open — after cmd48/49/53 crash boundary)
    test("resolve_unknown_receipt_open") {
        val port = ScriptedPort(
            76 to "0\t1\t10",         // cmd76: IsOpen=1 (open receipt)
            255 to "0\tSOME_UNP"      // cmd255(UNP) still called
        )
        val state = DatecsRecovery(port, DUMMY_JOURNAL).resolve(PRE, "OP_UNP_42")
        check(state == ResolutionState.UNKNOWN) { "expected UNKNOWN for IsOpen=1, got $state" }
    }
    // UNKNOWN: response unavailable (DatecsFailure.Unknown — timeout/connection lost)
    test("resolve_unknown_response_unavailable") {
        val port = object : DatecsFiscalPort {
            override fun execute(cmd: Int, payload: ByteArray): DatecsResponse =
                throw DatecsFailure.Unknown("DATECS_TIMEOUT")
            override fun reachable() = false
        }
        val state = try {
            DatecsRecovery(port, DUMMY_JOURNAL).resolve(PRE, "OP_UNP_42")
        } catch (e: DatecsFailure.Unknown) {
            ResolutionState.UNKNOWN
        }
        check(state == ResolutionState.UNKNOWN) { "expected UNKNOWN for timeout, got $state" }
    }
    // UNKNOWN: IsOpen=0, number not greater, UNP doesn't match pre.unp or expectedUnp
    test("resolve_unknown_unp_mismatch") {
        val port = ScriptedPort(
            76 to "0\t0\t10",          // IsOpen=0, Number=10 (not > 10)
            255 to "0\tDIFFERENT_UNP"  // UNP doesn't match pre.unp or expectedUnp
        )
        val state = DatecsRecovery(port, DUMMY_JOURNAL).resolve(PRE, "OP_UNP_42")
        check(state == ResolutionState.UNKNOWN) { "expected UNKNOWN for UNP mismatch, got $state" }
    }
    // COMMITTED requires real number FROM FU (greater than pre, not synthetic)
    test("committed_requires_greater_number") {
        // Number=10 equals pre.receiptNumber → NOT committed (no increment means not executed)
        val port = ScriptedPort(
            76 to "0\t0\t10",
            255 to "0\tOP_UNP_42"
        )
        val state = DatecsRecovery(port, DUMMY_JOURNAL).resolve(PRE, "OP_UNP_42")
        check(state != ResolutionState.COMMITTED) { "must not be COMMITTED when number not greater" }
    }
    check(failures == 0) { "$failures DatecsRecovery vendor test failures" }
    println("actual Kotlin DatecsRecovery vendor tests: OK")
}
