package com.beeloy.fiscal.bluecash

/** State of the fiscal device receipt at the pre-open snapshot point (before cmd 48). */
data class PreOpenSnapshot(
    val isOpen: Int,
    val receiptNumber: Int,
    val unp: String,
    val lastFiscalDoc: Int
)

/** Result of resolving an UNKNOWN operation by reading the device after the fact. */
enum class ResolutionState { COMMITTED, NOT_COMMITTED, UNKNOWN }

/**
 * Reads FU state (cmd 76/255/60/62) and resolves UNKNOWN outcomes.
 * VG-04: snapshot before cmd 48, resolve after UNKNOWN, cancel orphan receipts.
 * VG-07: read FMnumber + nLastFiscalDoc + UNP after cmd 56 for evidence.
 * VG-09: read currency mode at activation.
 */
class DatecsRecovery(
    private val fiscal: DatecsFiscalPort,
    private val journal: TransactionJournal
) {
    /**
     * VG-04.1: Record pre-open state before cmd 48.
     * cmd 76 → IsOpen, Number; cmd 255 UNP + nLastFiscalDoc.
     */
    fun snapshot(operationId: String): PreOpenSnapshot {
        val r76 = fiscal.execute(76).requireOk(2)
        val f76 = DatecsText.decode(r76.data).split('\t')
        // f76[0]=ErrorCode(validated 0), f76[1]=IsOpen, f76[2]=Number
        val isOpen = f76.getOrElse(1) { "0" }.toIntOrNull() ?: 0
        val number = f76.getOrElse(2) { "0" }.toIntOrNull() ?: 0
        val unp = readParam("UNP")
        val lastDoc = readParam("nLastFiscalDoc").toIntOrNull() ?: 0
        val snap = PreOpenSnapshot(isOpen, number, unp, lastDoc)
        journal.append(operationId, "PRE_OPEN",
            "isOpen=$isOpen,number=$number,unp=$unp,lastDoc=$lastDoc")
        return snap
    }

    /**
     * VG-04.2: Resolve UNKNOWN after a sale attempt by reading current device state.
     * Returns COMMITTED, NOT_COMMITTED, or UNKNOWN.
     */
    fun resolve(expected: PreOpenSnapshot, expectedUnp: String): ResolutionState {
        val r76 = fiscal.execute(76).requireOk(2)
        val f76 = DatecsText.decode(r76.data).split('\t')
        // f76[0]=ErrorCode, f76[1]=IsOpen, f76[2]=Number
        val isOpen = f76.getOrElse(1) { "-1" }.toIntOrNull() ?: -1
        val number = f76.getOrElse(2) { "0" }.toIntOrNull() ?: 0
        val currentUnp = readParam("UNP")
        return when {
            isOpen == 0 && number > expected.receiptNumber && currentUnp == expectedUnp ->
                ResolutionState.COMMITTED
            isOpen == 0 && currentUnp == expected.unp ->
                ResolutionState.NOT_COMMITTED
            else -> ResolutionState.UNKNOWN
        }
    }

    /**
     * VG-04.3: Cancel any open receipt that doesn't belong to an active operation.
     * Called at startup and before cmd 48.
     */
    fun cancelOrphanIfNeeded(operationId: String): Boolean {
        val r76 = fiscal.execute(76).requireOk(2)
        val isOpen = DatecsText.decode(r76.data).split('\t').getOrElse(1) { "0" }.toIntOrNull() ?: 0
        if (isOpen == 0) return false
        return try {
            fiscal.execute(DatecsPayloads.CMD_CANCEL_RECEIPT, DatecsPayloads.cancelReceipt())
            journal.append(operationId, "ORPHAN_RECEIPT_CANCELED", "isOpen=$isOpen")
            true
        } catch (e: DatecsFailure) {
            journal.append(operationId, "ORPHAN_RECEIPT_CANCEL_FAILED", "error=${e.message}")
            false
        }
    }

    /**
     * VG-07.1: Read fiscal evidence after cmd 56 close.
     * Returns FiscalEvidence with docNumber, fiscalMemoryNumber, unp.
     */
    fun readEvidence(): FiscalEvidence {
        val docNumber = readParam("nLastFiscalDoc").toIntOrNull() ?: 0
        val fmNumber = readParam("FMnumber")
        val unp = readParam("UNP")
        return FiscalEvidence(docNumber, fmNumber, unp)
    }

    /**
     * VG-09.1: Read currency mode parameter at activation.
     * Datecs stores it in a vendor-specific parameter (DOC-V2 pending confirmation).
     * Returns 0=inactive, 1=BGN+EUR, 2=EUR+BGN, 3=EUR-only.
     */
    fun readCurrencyMode(): Int {
        return try {
            readParam("CurrencyMode").toIntOrNull() ?: 0
        } catch (e: DatecsFailure) {
            0
        }
    }

    private fun readParam(name: String): String {
        val resp = fiscal.execute(255, DatecsText.encode(name)).requireOk(2)
        // field[0]=ErrorCode, field[1]=value
        return DatecsText.decode(resp.data).split('\t').getOrElse(1) { "" }.trim()
    }
}

/** Fiscal document evidence built from cmd 255/62 after cmd 56. */
data class FiscalEvidence(
    val docNumber: Int,
    val fiscalMemoryNumber: String,
    val unp: String
)
