package com.beeloy.fiscal.bluecash

import java.nio.charset.StandardCharsets
import java.time.Instant
import java.util.Base64

data class ActivationClaims(
  val organizationId: String,
  val locationId: String,
  val deviceId: String,
  val appInstanceId: String,
  val audience: String,
  val expiresAtEpochSeconds: Long,
)

object ActivationToken {
  private fun string(json: String, key: String): String? =
    Regex("\\\"${Regex.escape(key)}\\\"\\s*:\\s*\\\"([^\\\"]+)\\\"").find(json)?.groupValues?.get(1)
  private fun number(json: String, key: String): Long? =
    Regex("\\\"${Regex.escape(key)}\\\"\\s*:\\s*([0-9]+)")
      .find(json)
      ?.groupValues
      ?.get(1)
      ?.toLongOrNull()

  fun parseAndBind(
    jwt: String,
    expectedOrganization: String,
    expectedLocation: String,
    expectedDevice: String,
    now: Instant = Instant.now()
  ): ActivationClaims {
    val parts = jwt.split('.')
    require(parts.size == 3 && parts.all { it.isNotBlank() }) { "ACTIVATION_JWT_INVALID" }
    val payload = String(Base64.getUrlDecoder().decode(parts[1]), StandardCharsets.UTF_8)
    val claims =
      ActivationClaims(
        organizationId = string(payload, "organization_id") ?: error("ORGANIZATION_ID_MISSING"),
        locationId = string(payload, "location_id") ?: error("LOCATION_ID_MISSING"),
        deviceId = string(payload, "device_id") ?: error("DEVICE_ID_MISSING"),
        appInstanceId = string(payload, "app_instance_id") ?: error("APP_INSTANCE_ID_MISSING"),
        audience = string(payload, "aud") ?: error("AUDIENCE_MISSING"),
        expiresAtEpochSeconds = number(payload, "exp") ?: error("EXPIRY_MISSING"),
      )
    require(claims.audience == "beefiscal-bluecash-activation") { "ACTIVATION_AUDIENCE_INVALID" }
    require(claims.organizationId == expectedOrganization) { "ORGANIZATION_BINDING_MISMATCH" }
    require(claims.locationId == expectedLocation) { "LOCATION_BINDING_MISMATCH" }
    require(claims.deviceId == expectedDevice) { "DEVICE_BINDING_MISMATCH" }
    require(now.epochSecond < claims.expiresAtEpochSeconds) { "ACTIVATION_JWT_EXPIRED" }
    return claims
  }
}
