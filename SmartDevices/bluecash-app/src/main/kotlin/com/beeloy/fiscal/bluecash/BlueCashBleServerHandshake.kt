package com.beeloy.fiscal.bluecash

import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.security.SecureRandom
import java.time.Instant
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

data class BlueCashBleBinding(val tenantId:String,val locationId:String,val registerId:String,val edgeId:String,val fiscalDeviceId:String,val generation:Long=1)

/** Peripheral side of HELLO → CHALLENGE → AUTH_PROOF → READY. */
class BlueCashBleServerHandshake(private val ticketSigningKey:ByteArray,private val expected:BlueCashBleBinding,private val clock:()->Instant={Instant.now()},private val random:SecureRandom=SecureRandom()){
 enum class State{NEW,CHALLENGED,READY,FAILED}
 var state=State.NEW;private set
 var frames:BlueCashBleFrameSession?=null;private set
 var ticket:BlueCashBleTicket?=null;private set
 private var secret:ByteArray?=null;private var clientNonce:ByteArray?=null;private var edgeNonce:ByteArray?=null
 private val b64d=Base64.getUrlDecoder();private val b64e=Base64.getUrlEncoder().withoutPadding()
 fun hello(raw:ByteArray):ByteArray {try{require(state==State.NEW){"BLE_HANDSHAKE_STATE"};val root=BlueCashCanonicalCbor.decodeMap(raw);control(root,"HELLO",0);val payload=root["payload"] as? Map<*,*>?:error("BLE_HELLO_PAYLOAD");val signed=payload["ticket"] as? String?:error("BLE_HELLO_TICKET");val nonce=b64d.decode(payload["client_nonce"] as? String?:error("BLE_HELLO_NONCE"));val clientPublic=b64d.decode(payload["ephemeral_public_key"] as? String?:error("BLE_HELLO_KEY"));val parsed=BlueCashBleHandshakeCrypto.parseAndVerifyTicket(signed,ticketSigningKey,clock());require(parsed.sessionId==root["session_id"]&&parsed.tenantId==expected.tenantId&&parsed.locationId==expected.locationId&&parsed.registerId==expected.registerId&&parsed.edgeId==expected.edgeId&&parsed.fiscalDeviceId==expected.fiscalDeviceId){"BLE_TICKET_BINDING"};require(nonce.size>=16&&clientPublic.size==32){"BLE_HELLO_FIELDS"};val pair=BlueCashBleHandshakeCrypto.keyPair(random);val serverNonce=ByteArray(16).also{random.nextBytes(it)};val derived=BlueCashBleHandshakeCrypto.deriveServerSecret(pair,clientPublic,signed,nonce,serverNonce,parsed);ticket=parsed;clientNonce=nonce;edgeNonce=serverNonce;secret=derived;frames=BlueCashBleFrameSession.edge(derived,parsed.sessionId);state=State.CHALLENGED;return BlueCashCanonicalCbor.encode(mapOf("type" to "CHALLENGE","protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to parsed.sessionId,"counter" to 0,"payload" to mapOf("edge_nonce" to b64e.encodeToString(serverNonce),"ephemeral_public_key" to b64e.encodeToString(pair.publicKey),"max_chunk" to 132,"window" to 4)))}catch(e:Throwable){state=State.FAILED;throw e}}
 fun authenticate(raw:ByteArray):ByteArray {try{require(state==State.CHALLENGED){"BLE_HANDSHAKE_STATE"};val root=BlueCashCanonicalCbor.decodeMap(raw);control(root,"AUTH_PROOF",1);val current=ticket?:error("BLE_HANDSHAKE_STATE");require(root["session_id"]==current.sessionId){"BLE_SESSION_BINDING"};val payload=root["payload"] as? Map<*,*>?:error("BLE_AUTH_PAYLOAD");val ciphertext=b64d.decode(payload["ciphertext"] as? String?:error("BLE_AUTH_CIPHERTEXT"));val aad=BlueCashCanonicalCbor.encode(mapOf("counter" to 1,"protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to current.sessionId,"type" to "AUTH_PROOF"));val nonce=sha("BeeFiscal BLE auth nonce|${current.sessionId}".toByteArray(StandardCharsets.UTF_8)).copyOf(12);val plain=decrypt(secret?:error("BLE_HANDSHAKE_STATE"),nonce,aad,ciphertext);val proof=BlueCashCanonicalCbor.decodeMap(plain);require(proof["session_id"]==current.sessionId){"BLE_AUTH_BINDING"};val supplied=b64d.decode(proof["proof"] as? String?:error("BLE_AUTH_PROOF"));val expectedProof=BlueCashBleHandshakeCrypto.proof(secret!!,current,clientNonce!!,edgeNonce!!);require(MessageDigest.isEqual(supplied,expectedProof)){"BLE_AUTH_PROOF"};state=State.READY;return BlueCashCanonicalCbor.encode(mapOf("type" to "READY","protocol_version" to BlueCashBleHandshakeCrypto.PROTOCOL_VERSION,"session_id" to current.sessionId,"counter" to 1,"payload" to mapOf("next_counter" to 1,"max_chunk" to 132,"window" to 4)))}catch(e:Throwable){state=State.FAILED;secret?.fill(0);throw e}}
 fun close(){secret?.fill(0);secret=null;frames=null;state=State.FAILED}
 private fun control(v:Map<String,Any?>,type:String,counter:Long){require(v["type"]==type&&v["protocol_version"]==BlueCashBleHandshakeCrypto.PROTOCOL_VERSION&&(v["counter"] as? Number)?.toLong()==counter){"BLE_CONTROL_INVALID"}}
 private fun decrypt(key:ByteArray,nonce:ByteArray,aad:ByteArray,ciphertext:ByteArray)=Cipher.getInstance("AES/GCM/NoPadding").run{init(Cipher.DECRYPT_MODE,SecretKeySpec(key,"AES"),GCMParameterSpec(128,nonce));updateAAD(aad);doFinal(ciphertext)}
 private fun sha(v:ByteArray)=MessageDigest.getInstance("SHA-256").digest(v)
}
