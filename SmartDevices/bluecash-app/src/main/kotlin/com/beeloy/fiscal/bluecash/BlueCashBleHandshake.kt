package com.beeloy.fiscal.bluecash

import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.security.SecureRandom
import java.time.Instant
import java.util.Base64
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec
import org.bouncycastle.crypto.agreement.X25519Agreement
import org.bouncycastle.crypto.digests.SHA256Digest
import org.bouncycastle.crypto.generators.HKDFBytesGenerator
import org.bouncycastle.crypto.params.HKDFParameters
import org.bouncycastle.crypto.params.X25519PrivateKeyParameters
import org.bouncycastle.crypto.params.X25519PublicKeyParameters
import org.json.JSONObject

data class BlueCashBleTicket(
  val sessionId: String,
  val tenantId: String,
  val locationId: String,
  val registerId: String,
  val edgeId: String,
  val fiscalDeviceId: String,
  val clientPublicKey: String,
  val scopes: Set<String>,
  val fencingToken: Long,
  val expiresAt: Instant
)

data class BlueCashX25519KeyPair(val privateKey: ByteArray, val publicKey: ByteArray)

object BlueCashBleHandshakeCrypto {
  const val PROTOCOL_VERSION = "2026-08-07"
  private val b64 = Base64.getUrlDecoder()
  fun keyPair(random: SecureRandom = SecureRandom()): BlueCashX25519KeyPair {
    val private = X25519PrivateKeyParameters(random)
    return BlueCashX25519KeyPair(private.encoded, private.generatePublicKey().encoded)
  }
  fun parseAndVerifyTicket(
    raw: String,
    signingKey: ByteArray,
    now: Instant = Instant.now()
  ): BlueCashBleTicket {
    require(signingKey.size >= 16) { "BLE_TICKET_KEY_INVALID" }
    val wrapper = JSONObject(String(b64.decode(raw), StandardCharsets.UTF_8))
    val payloadEncoded = wrapper.getString("payload")
    val payload = b64.decode(payloadEncoded)
    val supplied = b64.decode(wrapper.getString("signature"))
    val expected =
      Mac.getInstance("HmacSHA256").run {
        init(SecretKeySpec(signingKey, "HmacSHA256"))
        doFinal(payload)
      }
    require(MessageDigest.isEqual(expected, supplied)) { "BLE_TICKET_SIGNATURE" }
    val v = JSONObject(String(payload, StandardCharsets.UTF_8))
    val scopes = v.getJSONArray("Scopes")
    val ticket =
      BlueCashBleTicket(
        v.getString("SessionID"),
        v.getString("TenantID"),
        v.getString("LocationID"),
        v.getString("RegisterID"),
        v.getString("DeviceID"),
        v.getString("FiscalDeviceID"),
        v.getString("ClientPublicKey"),
        (0 until scopes.length()).map { scopes.getString(it) }.toSet(),
        v.getLong("FencingToken"),
        Instant.parse(v.getString("ExpiresAt"))
      )
    require(now.isBefore(ticket.expiresAt)) { "BLE_TICKET_EXPIRED" }
    require("fiscal.execute" in ticket.scopes && ticket.fencingToken >= 1) { "BLE_TICKET_SCOPE" }
    return ticket
  }
  fun deriveSecret(
    pair: BlueCashX25519KeyPair,
    peerPublic: ByteArray,
    ticketRaw: String,
    clientNonce: ByteArray,
    edgeNonce: ByteArray,
    ticket: BlueCashBleTicket
  ): ByteArray {
    require(
      Base64.getUrlEncoder().withoutPadding().encodeToString(pair.publicKey) ==
        ticket.clientPublicKey
    ) {
      "BLE_CLIENT_KEY_BINDING"
    }
    return derive(pair, peerPublic, ticketRaw, clientNonce, edgeNonce, ticket)
  }
  fun deriveServerSecret(
    edgePair: BlueCashX25519KeyPair,
    clientPublic: ByteArray,
    ticketRaw: String,
    clientNonce: ByteArray,
    edgeNonce: ByteArray,
    ticket: BlueCashBleTicket
  ): ByteArray {
    require(
      Base64.getUrlEncoder().withoutPadding().encodeToString(clientPublic) == ticket.clientPublicKey
    ) {
      "BLE_CLIENT_KEY_BINDING"
    }
    return derive(edgePair, clientPublic, ticketRaw, clientNonce, edgeNonce, ticket)
  }
  private fun derive(
    pair: BlueCashX25519KeyPair,
    peerPublic: ByteArray,
    ticketRaw: String,
    clientNonce: ByteArray,
    edgeNonce: ByteArray,
    ticket: BlueCashBleTicket
  ): ByteArray {
    require(
      pair.privateKey.size == 32 &&
        pair.publicKey.size == 32 &&
        peerPublic.size == 32 &&
        clientNonce.size >= 16 &&
        edgeNonce.size >= 16
    ) {
      "BLE_HANDSHAKE_FIELDS"
    }
    val agreement = X25519Agreement().apply { init(X25519PrivateKeyParameters(pair.privateKey, 0)) }
    val shared = ByteArray(agreement.agreementSize)
    agreement.calculateAgreement(X25519PublicKeyParameters(peerPublic, 0), shared, 0)
    val ticketDigest = sha(ticketRaw.toByteArray(StandardCharsets.UTF_8))
    val salt = sha(ticketDigest + clientNonce + edgeNonce)
    val context =
      "${ticket.tenantId}|${ticket.locationId}|${ticket.registerId}|${ticket.edgeId}|${ticket.fiscalDeviceId}|${ticket.sessionId}|$PROTOCOL_VERSION"
    val out = ByteArray(32)
    HKDFBytesGenerator(SHA256Digest()).apply {
      init(
        HKDFParameters(
          shared,
          salt,
          "BeeFiscal BLE v1|$context".toByteArray(StandardCharsets.UTF_8)
        )
      )
      generateBytes(out, 0, out.size)
    }
    shared.fill(0)
    return out
  }
  fun proof(
    secret: ByteArray,
    ticket: BlueCashBleTicket,
    clientNonce: ByteArray,
    edgeNonce: ByteArray
  ) = sha(secret + ticket.sessionId.toByteArray(StandardCharsets.UTF_8) + clientNonce + edgeNonce)
  private fun sha(v: ByteArray) = MessageDigest.getInstance("SHA-256").digest(v)
}
