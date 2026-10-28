package com.beeloy.fiscal.runtime

/**
 * Vendor-neutral interface for keystore-backed secret storage.
 *
 * Invariant 8: implementations must never expose values in logs, crash reports,
 * or over unencrypted transports. Tests must not use real keys.
 */
interface SecretVault {
    /** Store a named secret value. Pass an empty string to clear a secret. */
    fun put(key: String, value: String)

    /** Retrieve a named secret value, or null if not set. */
    fun get(key: String): String?

    /** Returns true only when all required secrets are present and non-blank. */
    fun isConfigured(): Boolean
}
