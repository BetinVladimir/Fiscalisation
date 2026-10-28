package com.beeloy.fiscal.bluecash

/** Protocol constants from the Datecs/Daisy spec. */
object DatecsLimits {
    const val MAX_RECEIPT_LINES = 99           // §5.3
    const val MAX_NAME_LEN = 72                // §5.3 text field
    const val MAX_PAYMENT_SLOTS = 4            // §5.4
    const val MIN_SEQ: Int = 0x20              // §4.1
    const val MAX_SEQ_DATECS: Int = 0x7F       // §4.1 Datecs
    const val MAX_SEQ_DAISY: Int = 0xFF        // §4.1 Daisy
    const val LEN_OFFSET_DATECS: Int = 0x2A   // §4.2
    const val LEN_OFFSET_DAISY: Int = 0x24    // §4.2

    /**
     * VG-22: Commands listed in the Datecs PM as "not used in BC-50".
     * §4.1(33), §4.2(35), §4.10(47), §4.22(61), §4.24(63), §4.35(80), §4.54(106).
     * Emitting these commands on a BC-50 device is a protocol error.
     */
    val BC50_UNSUPPORTED_CMDS: Set<Int> = setOf(33, 35, 47, 61, 63, 80, 106)
}
