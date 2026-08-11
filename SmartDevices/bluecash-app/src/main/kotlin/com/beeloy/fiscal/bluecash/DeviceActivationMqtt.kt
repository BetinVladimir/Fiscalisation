package com.beeloy.fiscal.bluecash

import org.eclipse.paho.client.mqttv3.IMqttDeliveryToken
import org.eclipse.paho.client.mqttv3.MqttAsyncClient
import org.eclipse.paho.client.mqttv3.MqttCallbackExtended
import org.eclipse.paho.client.mqttv3.MqttConnectOptions
import org.eclipse.paho.client.mqttv3.MqttMessage
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.security.KeyStore
import java.security.PrivateKey
import java.security.Signature
import java.security.cert.Certificate
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate
import java.util.Base64
import java.util.UUID
import javax.net.ssl.KeyManager
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509KeyManager

/** Completes activation over mutually authenticated MQTT and exposes ACTIVE only after CA-signed acknowledgement. */
class DeviceActivationMqtt(
    private val identity: AndroidDeviceIdentity,
    private val credentialStore: DeviceCredentialStore,
    private val binding: ActivatedBinding,
    private val onActive: (ActivatedBinding) -> Unit,
) : MqttCallbackExtended {
    private val request = JSONObject(credentialStore.get("pending") ?: error("NO_PENDING_ACTIVATION"))
    private val client = MqttAsyncClient(binding.mqttTlsUri, "beefiscal-activation-${binding.deviceId}", null)
    private val ackTopic = "beefiscal/v1/devices/${binding.deviceId}/activation/ack"

    fun start() {
        require(binding.mqttTlsUri.startsWith("ssl://")) { "MQTT_MTLS_REQUIRED" }
        client.setCallback(this)
        val options = MqttConnectOptions().apply { isCleanSession = false; isAutomaticReconnect = true; socketFactory = DeviceTLS.socketFactory(binding); connectionTimeout = 15; keepAliveInterval = 30 }
        client.connect(options).waitForCompletion(15_000)
    }
    fun stop() { if (client.isConnected) client.disconnect().waitForCompletion(5_000); client.close() }
    override fun connectComplete(reconnect: Boolean, serverURI: String?) {
        client.subscribe(ackTopic, 1).waitForCompletion(10_000)
        val nonce = UUID.randomUUID().toString()
        val id = request.getString("activation_request_id")
        val proof = "activate\n$id\n${binding.credentialId}\n${binding.bindingVersion}\n$nonce"
        val body = JSONObject().put("activation_request_id", id).put("credential_id", binding.credentialId).put("nonce", nonce).put("signature", identity.sign(proof))
        client.publish("beefiscal/v1/devices/${binding.deviceId}/activation", MqttMessage(body.toString().toByteArray()).apply { qos = 1; isRetained = false }).waitForCompletion(10_000)
    }
    override fun messageArrived(topic: String, message: MqttMessage) {
        require(topic == ackTopic) { "ACTIVATION_ACK_TOPIC_MISMATCH" }
        val ack = JSONObject(String(message.payload)); val signature = ack.remove("signature") as? String ?: error("ACTIVATION_ACK_SIGNATURE_MISSING")
        require(ack.getString("activation_request_id") == request.getString("activation_request_id") && ack.getString("device_instance_id") == binding.deviceId && ack.getString("state") == "ACTIVE" && ack.getLong("binding_version") == binding.bindingVersion && ack.getString("organization_id") == binding.organizationId && ack.getString("location_id") == binding.locationId && ack.getString("register_id") == binding.registerId) { "ACTIVATION_ACK_BINDING_MISMATCH" }
        require(verifyCA(CanonicalJson.encode(ack).toByteArray(), signature)) { "ACTIVATION_ACK_SIGNATURE_INVALID" }
        credentialStore.put("active", JSONObject().put("binding", JSONObject(credentialStore.get("binding")!!)).put("activated_at", java.time.Instant.now().toString()).toString())
        onActive(binding)
    }
    override fun connectionLost(cause: Throwable?) = Unit
    override fun deliveryComplete(token: IMqttDeliveryToken?) = Unit

    private fun verifyCA(unsigned: ByteArray, encoded: String): Boolean = DeviceTLS.verifyCA(binding, unsigned, encoded)
}

object DeviceTLS {
    fun socketFactory(binding: ActivatedBinding): javax.net.ssl.SSLSocketFactory {
        val chain = certificates(binding.certificatePem + "\n" + binding.caChainPem)
        val ca = certificates(binding.caChainPem).first()
        val trustStore = KeyStore.getInstance(KeyStore.getDefaultType()).apply { load(null); setCertificateEntry("beefiscal-device-ca", ca) }
        val trust = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm()).apply { init(trustStore) }
        val manager = object : X509KeyManager {
            override fun getClientAliases(keyType: String?, issuers: Array<java.security.Principal>?): Array<String> = arrayOf(AndroidDeviceIdentity.KEY_ALIAS)
            override fun chooseClientAlias(keyType: Array<String>?, issuers: Array<java.security.Principal>?, socket: java.net.Socket?): String = AndroidDeviceIdentity.KEY_ALIAS
            override fun getServerAliases(keyType: String?, issuers: Array<java.security.Principal>?): Array<String>? = null
            override fun chooseServerAlias(keyType: String?, issuers: Array<java.security.Principal>?, socket: java.net.Socket?): String? = null
            override fun getCertificateChain(alias: String?): Array<X509Certificate> = chain.toTypedArray()
            override fun getPrivateKey(alias: String?): PrivateKey = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.getKey(AndroidDeviceIdentity.KEY_ALIAS, null) as PrivateKey
        }
        return SSLContext.getInstance("TLSv1.3").apply { init(arrayOf<KeyManager>(manager), trust.trustManagers, null) }.socketFactory
    }
    fun verifyCA(binding: ActivatedBinding, unsigned: ByteArray, encoded: String): Boolean {
        val ca = certificates(binding.caChainPem).first()
        return Signature.getInstance(if (ca.publicKey.algorithm == "RSA") "SHA256withRSA" else "SHA256withECDSA").run { initVerify(ca.publicKey); update(unsigned); verify(Base64.getUrlDecoder().decode(encoded)) }
    }
    private fun certificates(pem: String): List<X509Certificate> = CertificateFactory.getInstance("X.509").generateCertificates(ByteArrayInputStream(pem.toByteArray())).map { it as X509Certificate }
}
