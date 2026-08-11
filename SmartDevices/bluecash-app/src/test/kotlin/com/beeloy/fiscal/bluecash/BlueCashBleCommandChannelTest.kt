package com.beeloy.fiscal.bluecash

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test
import java.security.MessageDigest
import java.time.Instant
import java.util.Base64
import java.util.UUID
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

class BlueCashBleCommandChannelTest {
 @Test fun `authenticated encrypted command reaches one executor and returns encrypted result`() {val signing="0123456789abcdef0123456789abcdef".toByteArray();val b64=Base64.getUrlEncoder().withoutPadding();val clientPair=BlueCashBleHandshakeCrypto.keyPair();val payload=JSONObject().put("SessionID","s1").put("TenantID","t1").put("LocationID","l1").put("RegisterID","r1").put("DeviceID","e1").put("FiscalDeviceID","d1").put("AppInstanceID","a1").put("ActorSubject","u1").put("ClientPublicKey",b64.encodeToString(clientPair.publicKey)).put("Scopes",JSONArray().put("fiscal.execute")).put("FencingToken",1).put("ExpiresAt","2030-01-01T00:00:00Z").put("Nonce","n").toString().toByteArray();val sig=Mac.getInstance("HmacSHA256").run{init(SecretKeySpec(signing,"HmacSHA256"));doFinal(payload)};val ticket=b64.encodeToString(JSONObject().put("payload",b64.encodeToString(payload)).put("signature",b64.encodeToString(sig)).toString().toByteArray());val cn=ByteArray(16){4};var calls=0;val server=BlueCashBleServerHandshake(signing,BlueCashBleBinding("t1","l1","r1","e1","d1"),clock={Instant.parse("2026-08-11T00:00:00Z")});val channel=BlueCashBleCommandChannel(server,{intent->calls++;mapOf("operation_id" to intent["intent_id"],"state" to "FISCALIZED","version" to 1)})
  val hello=BlueCashCanonicalCbor.encode(mapOf("type" to "HELLO","protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to "s1","counter" to 0,"payload" to mapOf("ticket" to ticket,"client_nonce" to b64.encodeToString(cn),"ephemeral_public_key" to b64.encodeToString(clientPair.publicKey))));val challenge=BlueCashCanonicalCbor.decodeMap(channel.control(hello));val cp=challenge["payload"] as Map<*,*>;val en=Base64.getUrlDecoder().decode(cp["edge_nonce"] as String);val edgePublic=Base64.getUrlDecoder().decode(cp["ephemeral_public_key"] as String);val parsed=BlueCashBleHandshakeCrypto.parseAndVerifyTicket(ticket,signing,Instant.parse("2026-08-11T00:00:00Z"));val secret=BlueCashBleHandshakeCrypto.deriveSecret(clientPair,edgePublic,ticket,cn,en,parsed);val proof=BlueCashCanonicalCbor.encode(mapOf("proof" to b64.encodeToString(BlueCashBleHandshakeCrypto.proof(secret,parsed,cn,en)),"session_id" to "s1"));val aad=BlueCashCanonicalCbor.encode(mapOf("counter" to 1,"protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to "s1","type" to "AUTH_PROOF"));val nonce=MessageDigest.getInstance("SHA-256").digest("BeeFiscal BLE auth nonce|s1".toByteArray()).copyOf(12);val encrypted=Cipher.getInstance("AES/GCM/NoPadding").run{init(Cipher.ENCRYPT_MODE,SecretKeySpec(secret,"AES"),GCMParameterSpec(128,nonce));updateAAD(aad);doFinal(proof)};channel.control(BlueCashCanonicalCbor.encode(mapOf("type" to "AUTH_PROOF","protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to "s1","counter" to 1,"payload" to mapOf("ciphertext" to b64.encodeToString(encrypted)))))
  val clientFrames=BlueCashBleFrameSession.client(secret,"s1");val id=UUID.fromString("550e8400-e29b-41d4-a716-446655440101");val responses=channel.command(clientFrames.seal(id,0,1,0,BlueCashCanonicalCbor.encode(mapOf("intent_id" to id.toString(),"action" to "PAYMENT"))));val result=BlueCashCanonicalCbor.decodeMap(clientFrames.open(responses.single()).plaintext);assertEquals("FISCALIZED",result["state"]);assertEquals(1,calls)
 }
}
