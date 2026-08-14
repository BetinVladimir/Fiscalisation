package com.beeloy.fiscal.bluecash

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyInfo
import android.security.keystore.KeyProperties
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.security.KeyFactory
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.MessageDigest
import java.security.Signature
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec
import java.util.Base64
import java.util.UUID
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

data class PendingActivation(val requestId: String, val requestSecret: String, val userCode: String, val verificationUri: String, val expiresAt: String)
data class ActivatedBinding(val credentialId: String, val certificatePem: String, val caChainPem: String, val mqttTlsUri: String, val mqttWssUri: String?, val deviceId: String, val organizationId: String, val locationId: String, val registerId: String, val roles: List<String>, val bindingVersion: Long, val commandHmacKey: String, val syncAckHmacKey: String, val bleTicketHmacKey: String, val unpPrefix: String, val unpRangeStart: Long, val unpRangeEnd: Long, val localTokenIssuer:String?=null,val localTokenSigningKID:String?=null,val localTokenPublicKeyDERBase64:String?=null,val spaDeploymentDescriptorURL:String?=null,val spaDeploymentSigningKID:String?=null,val spaDeploymentPublicKeyDERBase64:String?=null)

/** Owns the non-exportable signing identity used for bootstrap, MQTT mTLS and journal signatures. */
class AndroidDeviceIdentity(private val context: Context) : TransactionSigner {
    companion object { const val KEY_ALIAS = "beefiscal.device.identity.v1"; private const val PREFS = "beefiscal_device_identity" }
    private val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
    val deviceInstanceId: String by lazy {
        val preferences = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        preferences.getString("device_instance_id", null) ?: UUID.randomUUID().toString().also { preferences.edit().putString("device_instance_id", it).apply() }
    }

    fun ensureKey(attestationChallenge: ByteArray) {
        if (store.containsAlias(KEY_ALIAS)) return
        fun generate(strongBox: Boolean) {
            val builder = KeyGenParameterSpec.Builder(KEY_ALIAS, KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY)
                .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                .setDigests(KeyProperties.DIGEST_SHA256)
                .setUserAuthenticationRequired(false)
                .setAttestationChallenge(attestationChallenge)
            if (android.os.Build.VERSION.SDK_INT >= 28) builder.setIsStrongBoxBacked(strongBox)
            KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore").run { initialize(builder.build()); generateKeyPair() }
        }
        if (android.os.Build.VERSION.SDK_INT >= 28) runCatching { generate(true) }.getOrElse { generate(false) } else generate(false)
        check(isHardwareBacked()) { "HARDWARE_BACKED_KEY_REQUIRED" }
    }

    fun isHardwareBacked(): Boolean {
        val privateKey = store.getKey(KEY_ALIAS, null) ?: return false
        val info = KeyFactory.getInstance(privateKey.algorithm, "AndroidKeyStore").getKeySpec(privateKey, KeyInfo::class.java)
        return if (android.os.Build.VERSION.SDK_INT >= 31) info.securityLevel != android.security.keystore.KeyProperties.SECURITY_LEVEL_SOFTWARE else info.isInsideSecureHardware
    }

    fun publicJwk(): JSONObject {
        val key = store.getCertificate(KEY_ALIAS).publicKey as ECPublicKey
        fun coordinate(value: java.math.BigInteger): String = Base64.getUrlEncoder().withoutPadding().encodeToString(value.toByteArray().let { bytes -> when { bytes.size == 32 -> bytes; bytes.size == 33 && bytes[0] == 0.toByte() -> bytes.copyOfRange(1, 33); bytes.size < 32 -> ByteArray(32 - bytes.size) + bytes; else -> error("INVALID_P256_COORDINATE") } })
        return JSONObject().put("kty", "EC").put("crv", "P-256").put("x", coordinate(key.w.affineX)).put("y", coordinate(key.w.affineY))
    }

    fun sign(raw: String): String = Signature.getInstance("SHA256withECDSA").run {
        initSign(store.getKey(KEY_ALIAS, null) as java.security.PrivateKey); update(raw.toByteArray(Charsets.UTF_8)); Base64.getUrlEncoder().withoutPadding().encodeToString(sign())
    }
    override val keyId: String get() { val digest = MessageDigest.getInstance("SHA-256").digest(CanonicalJson.encode(publicJwk()).toByteArray()); return Base64.getUrlEncoder().withoutPadding().encodeToString(digest) }
    override fun sign(hash: ByteArray): ByteArray = Signature.getInstance("SHA256withECDSA").run { initSign(store.getKey(KEY_ALIAS, null) as java.security.PrivateKey); update(hash); EcdsaP1363.fromDer(sign()) }
}

/** Encrypts bootstrap secrets at rest with an Android Keystore AES-GCM key. */
class DeviceCredentialStore(private val context: Context) {
    companion object { private const val ALIAS = "beefiscal.bootstrap.storage.v1"; private const val PREFS = "beefiscal_bootstrap" }
    private val preferences = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(ALIAS, null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT).setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build()); generateKey()
        }
    }
    fun put(name: String, value: String) {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, key()) }
        val encoded = Base64.getEncoder().encodeToString(cipher.iv + cipher.doFinal(value.toByteArray()))
        preferences.edit().putString(name, encoded).apply()
    }
    fun get(name: String): String? = preferences.getString(name, null)?.let { encoded ->
        val bytes = Base64.getDecoder().decode(encoded); Cipher.getInstance("AES/GCM/NoPadding").run { init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes.copyOfRange(0, 12))); String(doFinal(bytes.copyOfRange(12, bytes.size))) }
    }
    fun clear() = preferences.edit().clear().apply()
}

class DeviceBootstrapClient(private val baseUrl: String, private val identity: AndroidDeviceIdentity, private val secrets: DeviceCredentialStore) {
    private fun call(path: String, body: JSONObject): JSONObject {
        require(baseUrl.startsWith("https://")) { "HTTPS_REQUIRED" }
        val connection = URL(baseUrl.trimEnd('/') + path).openConnection() as HttpURLConnection
        connection.requestMethod = "POST"; connection.connectTimeout = 15_000; connection.readTimeout = 15_000; connection.doOutput = true
        connection.setRequestProperty("Content-Type", "application/json"); connection.outputStream.use { it.write(body.toString().toByteArray()) }
        val stream = if (connection.responseCode in 200..299) connection.inputStream else connection.errorStream
        val response = stream?.bufferedReader()?.use { it.readText() }.orEmpty()
        if (connection.responseCode !in 200..299) error("BOOTSTRAP_HTTP_${connection.responseCode}")
        return JSONObject(response)
    }

    fun begin(vendor: String, model: String, serial: String, fmin: String, firmware: String, roles: List<String>): PendingActivation {
        val challenge = call("/device-bootstrap/v1/challenges", JSONObject().put("device_instance_id", identity.deviceInstanceId))
        val nonce = challenge.getString("challenge")
        identity.ensureKey(nonce.toByteArray())
        val jwk = identity.publicJwk()
        val capabilities = roles.sorted().joinToString(",")
        val digest = MessageDigest.getInstance("SHA-256").digest(capabilities.toByteArray()).joinToString("") { "%02x".format(it) }
        val proof = listOf("activate", challenge.getString("challenge_id"), nonce, identity.deviceInstanceId, CanonicalJson.encode(jwk), vendor.uppercase(), model, serial, fmin, firmware, digest, capabilities).joinToString("\n")
        val request = JSONObject().put("challenge_id", challenge.getString("challenge_id")).put("challenge", nonce).put("device_instance_id", identity.deviceInstanceId).put("device_public_key_jwk", jwk).put("vendor", vendor.uppercase()).put("model", model).put("serial", serial).put("fmin", fmin).put("firmware", firmware).put("capability_digest", digest).put("requested_roles", JSONArray(roles)).put("signature", identity.sign(proof))
        val created = call("/device-bootstrap/v1/activation-requests", request)
        secrets.put("pending", created.toString())
        return PendingActivation(created.getString("activation_request_id"), created.getString("request_secret"), created.getString("user_code"), created.getString("verification_uri"), created.getString("expires_at"))
    }

    fun pollCredential(): ActivatedBinding {
        val pending = JSONObject(secrets.get("pending") ?: error("NO_PENDING_ACTIVATION"))
        val id = pending.getString("activation_request_id"); val secret = pending.getString("request_secret"); val nonce = UUID.randomUUID().toString()
        val secretHash = MessageDigest.getInstance("SHA-256").digest(secret.toByteArray()).joinToString("") { "%02x".format(it) }
        val result = call("/device-bootstrap/v1/activation-requests/$id/credential", JSONObject().put("request_secret", secret).put("nonce", nonce).put("signature", identity.sign("credential\n$id\n$nonce\n$secretHash")))
        val roles = result.getJSONArray("roles").let { values -> (0 until values.length()).map { values.getString(it) } }
        val binding = ActivatedBinding(result.getString("credential_id"), result.getString("client_certificate_pem"), result.getString("ca_chain_pem"), result.getString("mqtt_tls_uri"), result.optString("mqtt_wss_uri").ifBlank { null }, result.getString("device_instance_id"), result.getString("organization_id"), result.getString("location_id"), result.getString("register_id"), roles, result.getLong("binding_version"), result.getString("command_hmac_key"), result.getString("sync_ack_hmac_key"), result.getString("ble_ticket_hmac_key"), result.getString("unp_prefix"), result.getLong("unp_range_start"), result.getLong("unp_range_end"),result.optString("local_token_issuer").ifBlank{null},result.optString("local_token_signing_kid").ifBlank{null},result.optString("local_token_public_key_der_base64").ifBlank{null},result.optString("spa_deployment_descriptor_url").ifBlank{null},result.optString("spa_deployment_signing_kid").ifBlank{null},result.optString("spa_deployment_public_key_der_base64").ifBlank{null})
        require(binding.deviceId == identity.deviceInstanceId) { "CREDENTIAL_DEVICE_MISMATCH" }
        val signedBinding = JSONObject()
            .put("binding_version", binding.bindingVersion)
            .put("ble_ticket_hmac_key", binding.bleTicketHmacKey)
            .put("command_hmac_key", binding.commandHmacKey)
            .put("credential_id", binding.credentialId)
            .put("device_instance_id", binding.deviceId)
            .put("location_id", binding.locationId)
            .put("mqtt_tls_uri", binding.mqttTlsUri)
            .put("organization_id", binding.organizationId)
            .put("register_id", binding.registerId)
            .put("roles", JSONArray(binding.roles))
            .put("sync_ack_hmac_key", binding.syncAckHmacKey)
        binding.mqttWssUri?.let { signedBinding.put("mqtt_wss_uri", it) }
        require(DeviceTLS.verifyCA(binding, CanonicalJson.encode(signedBinding).toByteArray(), result.getString("binding_signature"))) { "BINDING_SIGNATURE_INVALID" }
        secrets.put("binding", result.toString()); return binding
    }
}
