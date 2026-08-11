package com.beeloy.fiscal.bluecash

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.time.Instant
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

class BlueCashBleHandshakeTest {
 private val key="0123456789abcdef0123456789abcdef".toByteArray();private val b64=Base64.getUrlEncoder().withoutPadding()
 private fun ticket(clientPublic:String,tenant:String="tenant-1"):String {val payload=JSONObject().put("SessionID","session-1").put("TenantID",tenant).put("LocationID","location-1").put("RegisterID","register-1").put("DeviceID","edge-1").put("FiscalDeviceID","device-1").put("AppInstanceID","app-1").put("ActorSubject","actor-1").put("ClientPublicKey",clientPublic).put("Scopes",JSONArray().put("fiscal.execute")).put("FencingToken",7).put("ExpiresAt","2030-01-01T00:00:00Z").put("Nonce","n").toString().toByteArray(StandardCharsets.UTF_8);val signature=Mac.getInstance("HmacSHA256").run{init(SecretKeySpec(key,"HmacSHA256"));doFinal(payload)};return b64.encodeToString(JSONObject().put("payload",b64.encodeToString(payload)).put("signature",b64.encodeToString(signature)).toString().toByteArray())}
 @Test fun `ticket signature binding and x25519 secret are symmetric`() {val client=BlueCashBleHandshakeCrypto.keyPair();val edge=BlueCashBleHandshakeCrypto.keyPair();val raw=ticket(b64.encodeToString(client.publicKey));val parsed=BlueCashBleHandshakeCrypto.parseAndVerifyTicket(raw,key,Instant.parse("2026-08-11T00:00:00Z"));assertEquals("tenant-1",parsed.tenantId);val cn=ByteArray(16){1};val en=ByteArray(16){2};val clientSecret=BlueCashBleHandshakeCrypto.deriveSecret(client,edge.publicKey,raw,cn,en,parsed);val edgeSecret=BlueCashBleHandshakeCrypto.deriveServerSecret(edge,client.publicKey,raw,cn,en,parsed);assertArrayEquals(clientSecret,edgeSecret);assertEquals(32,BlueCashBleHandshakeCrypto.proof(clientSecret,parsed,cn,en).size)}
 @Test fun `tampered ticket is rejected`() {val pair=BlueCashBleHandshakeCrypto.keyPair();val raw=ticket(b64.encodeToString(pair.publicKey));val wrapper=JSONObject(String(Base64.getUrlDecoder().decode(raw))).put("signature",b64.encodeToString(ByteArray(32)));val changed=b64.encodeToString(wrapper.toString().toByteArray());assertThrows(IllegalArgumentException::class.java){BlueCashBleHandshakeCrypto.parseAndVerifyTicket(changed,key,Instant.parse("2026-08-11T00:00:00Z"))}}
 @Test fun `server completes authenticated canonical cbor handshake`() {
  val client=BlueCashBleHandshakeCrypto.keyPair();val signed=ticket(b64.encodeToString(client.publicKey));val cn=ByteArray(16){3}
  val hello=BlueCashCanonicalCbor.encode(mapOf("type" to "HELLO","protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to "session-1","counter" to 0,"payload" to mapOf("ticket" to signed,"client_nonce" to b64.encodeToString(cn),"ephemeral_public_key" to b64.encodeToString(client.publicKey))))
  val server=BlueCashBleServerHandshake(key,BlueCashBleBinding("tenant-1","location-1","register-1","edge-1","device-1"),clock={Instant.parse("2026-08-11T00:00:00Z")})
  val challenge=BlueCashCanonicalCbor.decodeMap(server.hello(hello));val payload=challenge["payload"] as Map<*,*>;val en=Base64.getUrlDecoder().decode(payload["edge_nonce"] as String);val edgePublic=Base64.getUrlDecoder().decode(payload["ephemeral_public_key"] as String);val parsed=BlueCashBleHandshakeCrypto.parseAndVerifyTicket(signed,key,Instant.parse("2026-08-11T00:00:00Z"));val secret=BlueCashBleHandshakeCrypto.deriveSecret(client,edgePublic,signed,cn,en,parsed)
  val proof=BlueCashCanonicalCbor.encode(mapOf("proof" to b64.encodeToString(BlueCashBleHandshakeCrypto.proof(secret,parsed,cn,en)),"session_id" to "session-1"));val aad=BlueCashCanonicalCbor.encode(mapOf("counter" to 1,"protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to "session-1","type" to "AUTH_PROOF"));val nonce=MessageDigest.getInstance("SHA-256").digest("BeeFiscal BLE auth nonce|session-1".toByteArray()).copyOf(12);val cipher=Cipher.getInstance("AES/GCM/NoPadding").run{init(Cipher.ENCRYPT_MODE,SecretKeySpec(secret,"AES"),GCMParameterSpec(128,nonce));updateAAD(aad);doFinal(proof)}
  val auth=BlueCashCanonicalCbor.encode(mapOf("type" to "AUTH_PROOF","protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to "session-1","counter" to 1,"payload" to mapOf("ciphertext" to b64.encodeToString(cipher))));val ready=BlueCashCanonicalCbor.decodeMap(server.authenticate(auth));assertEquals("READY",ready["type"]);assertEquals(BlueCashBleServerHandshake.State.READY,server.state)
 }
}
