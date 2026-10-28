package com.beeloy.fiscal.bluecash

/**
 * Error catalog from Datecs FP protocol specification §7.
 * Generated from spec; do not edit manually — use scripts/gen-datecs-error-catalog.py.
 */
object DatecsErrorCatalog {
    data class Entry(val code: Int, val message: String, val category: ErrorCategory)

    enum class ErrorCategory { GENERAL, FISCAL, PAPER, DEVICE }

    val entries: List<Entry> = listOf(
        Entry(-100000, "Syntax error", ErrorCategory.GENERAL),
        Entry(-100001, "Checksum error", ErrorCategory.GENERAL),
        Entry(-100002, "Time out", ErrorCategory.GENERAL),
        Entry(-100003, "Command not found", ErrorCategory.GENERAL),
        Entry(-100004, "Sub-command not found", ErrorCategory.GENERAL),
        Entry(-100010, "Fiscal device not ready", ErrorCategory.FISCAL),
        Entry(-100011, "Fiscal device locked", ErrorCategory.FISCAL),
        Entry(-100015, "Wrong number of open receipts", ErrorCategory.FISCAL),
        Entry(-100041, "Printer paper error", ErrorCategory.PAPER),
        Entry(-100042, "Printer cover open", ErrorCategory.DEVICE),
        Entry(-100047, "Another open receipt", ErrorCategory.FISCAL),
        Entry(-111015, "Another open receipt in fiscal memory", ErrorCategory.FISCAL),
        Entry(-111047, "Receipt counter exhausted", ErrorCategory.FISCAL)
    )

    private val byCode = entries.associateBy { it.code }
    fun lookup(code: Int): Entry? = byCode[code]
}
