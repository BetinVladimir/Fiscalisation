package com.beeloy.fiscal.bluecash

import android.content.Context

/**
 * Keystore-backed storage for operator secrets (password, till, operator number).
 * Secrets are encrypted with AES/GCM via DeviceCredentialStore — never stored in plaintext.
 * Invariant 8: secrets must never be printed, committed, or placed in tests/logs.
 */
class SecretStore(context: Context) {
  private val store = DeviceCredentialStore(context)

  fun putOperatorPassword(value: String) = store.put("operator_password", value)
  fun getOperatorPassword(): String? = store.get("operator_password")

  fun putOperatorNumber(value: Int) = store.put("operator_number", value.toString())
  fun getOperatorNumber(): Int? = store.get("operator_number")?.toIntOrNull()

  fun putTillNumber(value: Int) = store.put("till_number", value.toString())
  fun getTillNumber(): Int? = store.get("till_number")?.toIntOrNull()

  fun clear() {
    listOf("operator_password", "operator_number", "till_number").forEach {
      store.put(it, "")
    }
  }

  fun isConfigured(): Boolean =
    !getOperatorPassword().isNullOrBlank() &&
      getOperatorNumber() != null &&
      getTillNumber() != null
}
