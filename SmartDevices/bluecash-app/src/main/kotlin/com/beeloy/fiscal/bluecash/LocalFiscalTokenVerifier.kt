package com.beeloy.fiscal.bluecash

import java.security.KeyFactory
import java.security.Signature
import java.security.spec.X509EncodedKeySpec
import java.time.Instant
import java.util.Base64
import org.json.JSONArray
import org.json.JSONObject

data class LocalFiscalAuthority(
  val subject: String,
  val tenantId: String,
  val locationId: String,
  val registerId: String,
  val operatorId: String,
  val shiftId: String,
  val adapterDeviceId: String,
  val bindingGeneration: Long,
  val scopes: Set<String>,
  val expiresAt: Instant,
  val jti: String,
)

/**
 * Verifies issuer, audience, expiry and register/device binding before any local command executes.
 */
class LocalFiscalTokenVerifier(
  private val issuer: String,
  publicKeyDerBase64: String,
  private val keyId: String,
  private val tenantId: String,
  private val locationId: String,
  private val registerId: String,
  private val adapterId: String,
  private val generation: Long,
  private val clock: () -> Instant = { Instant.now() },
) {
  private val key =
    KeyFactory.getInstance("EC")
      .generatePublic(
        X509EncodedKeySpec(Base64.getDecoder().decode(publicKeyDerBase64)),
      )

  fun verify(raw: String, requiredScope: String): LocalFiscalAuthority {
    val parts = raw.split('.')
    require(parts.size == 3) { "TOKEN_FORMAT" }
    val header = JSONObject(String(Base64.getUrlDecoder().decode(parts[0])))
    require(header.getString("alg") == "ES256" && header.getString("kid") == keyId) {
      "TOKEN_HEADER"
    }
    val signature = Base64.getUrlDecoder().decode(parts[2])
    require(signature.size == 64) { "TOKEN_SIGNATURE" }
    val verifier = Signature.getInstance("SHA256withECDSA")
    verifier.initVerify(key)
    verifier.update("${parts[0]}.${parts[1]}".toByteArray())
    require(verifier.verify(p1363ToDer(signature))) { "TOKEN_SIGNATURE" }

    val body = JSONObject(String(Base64.getUrlDecoder().decode(parts[1])))
    val now = clock().epochSecond
    val audiences = audiences(body)
    val issuedAt = body.getLong("iat")
    require(
      body.getString("iss") == issuer &&
        "beeloy-local-fiscal-adapter" in audiences &&
        adapterId in audiences &&
        body.getLong("nbf") <= now + 30 &&
        body.getLong("exp") > now - 30 &&
        body.getLong("exp") - issuedAt <= 930,
    ) {
      "TOKEN_TIME_OR_AUDIENCE"
    }
    val scopes = body.getString("scope").split(' ').filter(String::isNotBlank).toSet()
    require(requiredScope in scopes) { "TOKEN_SCOPE" }
    require(
      body.getString("tenant_id") == tenantId &&
        body.getString("location_id") == locationId &&
        body.getString("register_id") == registerId &&
        body.getString("adapter_device_id") == adapterId &&
        body.getLong("binding_generation") == generation,
    ) {
      "TOKEN_BINDING"
    }
    return LocalFiscalAuthority(
      body.getString("sub"),
      tenantId,
      locationId,
      registerId,
      body.getString("operator_id"),
      body.getString("shift_id"),
      adapterId,
      generation,
      scopes,
      Instant.ofEpochSecond(body.getLong("exp")),
      body.getString("jti"),
    )
  }

  private fun audiences(value: JSONObject): Set<String> =
    when (val raw = value.get("aud")) {
      is String -> setOf(raw)
      is JSONArray -> (0 until raw.length()).map(raw::getString).toSet()
      else -> emptySet()
    }

  private fun p1363ToDer(p1363: ByteArray): ByteArray {
    fun positiveInteger(raw: ByteArray): ByteArray {
      val firstNonZero = raw.indexOfFirst { it != 0.toByte() }
      val stripped =
        if (firstNonZero < 0) byteArrayOf(0) else raw.copyOfRange(firstNonZero, raw.size)
      return if ((stripped[0].toInt() and 0x80) != 0) byteArrayOf(0) + stripped else stripped
    }
    val r = positiveInteger(p1363.copyOfRange(0, 32))
    val s = positiveInteger(p1363.copyOfRange(32, 64))
    val sequenceLength = 2 + r.size + 2 + s.size
    return byteArrayOf(0x30, sequenceLength.toByte(), 0x02, r.size.toByte()) +
      r +
      byteArrayOf(0x02, s.size.toByte()) +
      s
  }
}
