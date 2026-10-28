package expo.modules.beeloyexecutioncrypto

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.MessageDigest
import java.security.Signature
import java.security.spec.ECGenParameterSpec

/** No method exposes private key bytes. All operations are serialized, including revocation. */
class ExecutionKeyStore(context: Context) {
  private val preferences = context.getSharedPreferences("beeloy.execution.keys.v1", Context.MODE_PRIVATE)
  private fun alias(scope: String): String {
    require(scope.isNotEmpty() && scope.toByteArray(Charsets.UTF_8).size <= 4096) { "EXECUTION_SCOPE_INVALID" }
    val digest = MessageDigest.getInstance("SHA-256").digest(scope.toByteArray(Charsets.UTF_8))
    return "beeloy.execution.v1." + digest.joinToString("") { "%02x".format(it) }
  }
  private fun store() = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
  private fun checkActive(alias: String) {
    check(!preferences.getBoolean(alias, false)) { "AUTHENTICATED_SESSION_REVOKED" }
  }
  private fun publicKey(store: KeyStore, alias: String): String =
    Base64.encodeToString(store.getCertificate(alias).publicKey.encoded, Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING)
  fun prepare(scope: String): String = synchronized(lock) {
    val name = alias(scope)
    checkActive(name)
    val keyStore = store()
    if (!keyStore.containsAlias(name)) {
      KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore").apply {
        initialize(KeyGenParameterSpec.Builder(name, KeyProperties.PURPOSE_SIGN)
          .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
          .setDigests(KeyProperties.DIGEST_SHA256).build())
      }.generateKeyPair()
    }
    publicKey(keyStore, name)
  }
  fun sign(scope: String, expectedPublicKey: String, messageBase64: String): String = synchronized(lock) {
    val name = alias(scope)
    checkActive(name)
    val keyStore = store()
    check(keyStore.containsAlias(name) && publicKey(keyStore, name) == expectedPublicKey) { "EXECUTION_KEY_CHANGED" }
    require(messageBase64.length <= 131072) { "EXECUTION_MESSAGE_TOO_LARGE" }
    val message = Base64.decode(messageBase64, Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING)
    require(message.isNotEmpty() && message.size <= 65536) { "EXECUTION_MESSAGE_INVALID" }
    val signer = Signature.getInstance("SHA256withECDSA")
    signer.initSign(keyStore.getKey(name, null) as java.security.PrivateKey)
    signer.update(message)
    Base64.encodeToString(signer.sign(), Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING)
  }
  fun revoke(scope: String) = synchronized(lock) {
    val name = alias(scope)
    // Persist the tombstone before deletion: an interrupted revoke must never regenerate this scope.
    check(preferences.edit().putBoolean(name, true).commit()) { "EXECUTION_REVOCATION_PERSIST_FAILED" }
    store().deleteEntry(name)
  }
  companion object { private val lock = Any() }
}
