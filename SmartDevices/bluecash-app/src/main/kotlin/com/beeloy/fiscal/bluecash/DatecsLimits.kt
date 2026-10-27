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
}
