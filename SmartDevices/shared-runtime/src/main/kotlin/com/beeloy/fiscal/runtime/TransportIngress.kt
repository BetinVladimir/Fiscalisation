package com.beeloy.fiscal.runtime

/**
 * Vendor-neutral transport ingress interface (SMS-02.5).
 *
 * Multiple transports (MQTT, BLE, local HTTP) can receive the same command.
 * The ingress contract ensures that two simultaneous transports for the same operationId
 * produce exactly one execution (deduplication via OperationJournal.beginOperation).
 *
 * Smart S vendor HTTP is implemented by SMS-03/08; this interface must not assume a
 * specific HTTP SDK.
 */
interface TransportIngress {
    /** Transport type identifier, e.g. "MQTT", "BLE", "HTTP". */
    val transportId: String

    /** Start receiving commands and routing them to [dispatcher]. */
    fun start(dispatcher: IngressDispatcher)

    /** Stop receiving; pending operations must be preserved in the journal. */
    fun stop()
}

/**
 * Routes incoming commands through a shared OperationJournal, providing:
 *   - Deduplication: same operationId from two transports → one execution (invariant 4).
 *   - Durable pending state: beginOperation records PENDING before I/O starts.
 *   - Restart recovery: a PENDING entry from a previous run is visible on next start.
 *
 * @param journal  durable journal (must survive process death in production; InMemory for tests).
 * @param execute  vendor-specific command execution function (e.g. BlueCashCommandProcessor.sale).
 */
class IngressDispatcher(
    private val journal: OperationJournal,
    private val execute: (operationId: String, payload: Map<String, String>) -> Map<String, String>
) {
    /**
     * Routes a command from any transport.
     *
     * - If operationId is new: records PENDING, executes, commits/marks-unknown.
     * - If operationId already exists: returns the journalled terminal state (deduplication).
     *   Invariant 4: no second execute call is made.
     */
    fun dispatch(operationId: String, payload: Map<String, String>): Map<String, String> {
        // Check if already terminal
        val existing = journal.findEntry(operationId)
        if (existing != null) {
            return when (existing.state) {
                OperationJournal.State.COMMITTED -> mapOf(
                    "state" to "COMMITTED",
                    "operationId" to operationId,
                    "fiscalRef" to (existing.fiscalRef ?: "")
                )
                OperationJournal.State.UNKNOWN -> mapOf(
                    "state" to "UNKNOWN",
                    "operationId" to operationId,
                    "error_code" to "LOOKUP_REQUIRED"
                )
                OperationJournal.State.FAILED -> mapOf(
                    "state" to "FAILED",
                    "operationId" to operationId,
                    "error_code" to "PREVIOUSLY_FAILED"
                )
                OperationJournal.State.FISCAL_PENDING -> mapOf(
                    "state" to "FISCAL_PENDING",
                    "operationId" to operationId,
                    "error_code" to "STORNO_REQUIRED"
                )
                OperationJournal.State.PENDING -> mapOf(
                    "state" to "UNKNOWN",
                    "operationId" to operationId,
                    "error_code" to "RECOVERY_REQUIRED"
                )
            }
        }
        // Try to begin — another transport may win the race (invariant 4)
        if (!journal.beginOperation(operationId)) {
            // Lost race: return whatever the winner recorded
            return dispatch(operationId, payload)
        }
        // We own this operation — execute it
        return try {
            val result = execute(operationId, payload)
            val fiscalRef = result["fiscal_reference"]
            journal.commit(operationId, fiscalRef)
            result + mapOf("state" to "COMMITTED")
        } catch (e: Throwable) {
            journal.markUnknown(operationId)
            mapOf(
                "state" to "UNKNOWN",
                "operationId" to operationId,
                "error_code" to (e.message ?: "EXECUTE_FAILED")
            )
        }
    }
}
