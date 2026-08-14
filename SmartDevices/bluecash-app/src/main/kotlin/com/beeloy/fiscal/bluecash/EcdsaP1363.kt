package com.beeloy.fiscal.bluecash

/** Strict conversion of Android SHA256withECDSA DER output to IEEE P1363 P-256 r||s. */
object EcdsaP1363 {
    fun fromDer(der: ByteArray): ByteArray {
        require(der.size in 8..72 && der[0] == 0x30.toByte()) { "ECDSA_DER_SEQUENCE" }
        var p = 1
        val sequenceLength = length(der, p).also { p = it.second }.first
        require(p + sequenceLength == der.size) { "ECDSA_DER_LENGTH" }
        fun integer(): ByteArray {
            require(p < der.size && der[p++] == 0x02.toByte()) { "ECDSA_DER_INTEGER" }
            val parsed = length(der, p); val n = parsed.first; p = parsed.second
            require(n in 1..33 && p + n <= der.size) { "ECDSA_DER_INTEGER_LENGTH" }
            val raw = der.copyOfRange(p, p + n); p += n
            require(raw[0].toInt() and 0x80 == 0 && !(raw.size > 1 && raw[0] == 0.toByte() && raw[1].toInt() and 0x80 == 0)) { "ECDSA_DER_INTEGER_CANONICAL" }
            val unsigned = if (raw.size == 33) { require(raw[0] == 0.toByte()); raw.copyOfRange(1, 33) } else raw
            require(unsigned.size <= 32 && unsigned.any { it != 0.toByte() }) { "ECDSA_DER_INTEGER_RANGE" }
            return ByteArray(32 - unsigned.size) + unsigned
        }
        val r = integer(); val s = integer(); require(p == der.size) { "ECDSA_DER_TRAILING" }
        return r + s
    }
    private fun length(v: ByteArray, at: Int): Pair<Int, Int> {
        require(at < v.size) { "ECDSA_DER_LENGTH" }; val first = v[at].toInt() and 255
        if (first < 128) return first to at + 1
        require(first == 0x81 && at + 1 < v.size && (v[at + 1].toInt() and 255) >= 128) { "ECDSA_DER_LENGTH" }
        return (v[at + 1].toInt() and 255) to at + 2
    }
}
