import test from "node:test";
import assert from "node:assert/strict";
import {BleClientHandshake,BLE_PROTOCOL_VERSION} from "./bleHandshake.ts";
import type {ControlMessage} from "./bleHandshake.ts";
import {decodeStrict,encodeCanonical} from "./cbor.ts";
import {WebBleBootstrap} from "./webBle.ts";

function b64(v:Uint8Array){let s="";for(const x of v)s+=String.fromCharCode(x);return btoa(s).replaceAll("+","-").replaceAll("/","_").replaceAll("=","")}
function ticket(sessionId:string,clientPublicKey:string){const payload=b64(new TextEncoder().encode(JSON.stringify({SessionID:sessionId,TenantID:"tenant",RegisterID:"register",DeviceID:"device",ClientPublicKey:clientPublicKey,Scopes:["fiscal.execute"]})));return b64(new TextEncoder().encode(JSON.stringify({payload,signature:"test"})))}

test("encrypted client handshake gates fiscal execution until READY",async()=>{
 const h=new BleClientHandshake(),sessionId="session-web-1",clientPublicKey=h.prepareSessionPublicKey(),rawTicket=ticket(sessionId,clientPublicKey);
 const hello=decodeStrict<ControlMessage>(await h.start({ble_session_id:sessionId,signed_session_ticket:rawTicket,protocol_version:BLE_PROTOCOL_VERSION,expires_at:new Date(Date.now()+60_000).toISOString()}));
 assert.equal(hello.type,"HELLO");assert.equal(h.canExecuteFiscalCommand,false);assert.equal(typeof hello.payload.client_nonce,"string");assert.equal(typeof hello.payload.ephemeral_public_key,"string");
 const edge=await crypto.subtle.generateKey({name:"X25519"} as any,false,["deriveBits"]) as CryptoKeyPair;const edgePublic=new Uint8Array(await crypto.subtle.exportKey("raw",edge.publicKey));
 const challenge=encodeCanonical({type:"CHALLENGE",protocol_version:BLE_PROTOCOL_VERSION,session_id:sessionId,counter:0,payload:{edge_nonce:b64(crypto.getRandomValues(new Uint8Array(16))),ephemeral_public_key:b64(edgePublic),max_chunk:128,window:4}} satisfies ControlMessage);
 const auth=decodeStrict<ControlMessage>(await h.challenge(challenge));assert.equal(auth.type,"AUTH_PROOF");assert.equal(typeof auth.payload.ciphertext,"string");assert.equal(h.canExecuteFiscalCommand,false);
 h.ready(encodeCanonical({type:"READY",protocol_version:BLE_PROTOCOL_VERSION,session_id:sessionId,counter:1,payload:{next_expected_counter:1}} satisfies ControlMessage));assert.equal(h.canExecuteFiscalCommand,true);
});

test("handshake rejects expired session and READY replay",async()=>{
 const expired=new BleClientHandshake(),expiredKey=expired.prepareSessionPublicKey();await assert.rejects(expired.start({ble_session_id:"s",signed_session_ticket:ticket("s",expiredKey),protocol_version:BLE_PROTOCOL_VERSION,expires_at:new Date(Date.now()-1).toISOString()}),/BLE_SESSION_EXPIRED/);
 const fresh=new BleClientHandshake(),freshKey=fresh.prepareSessionPublicKey();await fresh.start({ble_session_id:"s2",signed_session_ticket:ticket("s2",freshKey),protocol_version:BLE_PROTOCOL_VERSION,expires_at:new Date(Date.now()+60_000).toISOString()});
 assert.throws(()=>fresh.ready(encodeCanonical({type:"READY",protocol_version:BLE_PROTOCOL_VERSION,session_id:"s2",counter:1,payload:{}} satisfies ControlMessage)),/BLE_HANDSHAKE_STATE/);
});

test("handshake requires the private key prepared before REST issuance",async()=>{
 const unprepared=new BleClientHandshake();await assert.rejects(unprepared.start({ble_session_id:"s3",signed_session_ticket:ticket("s3",b64(new Uint8Array(32))),protocol_version:BLE_PROTOCOL_VERSION,expires_at:new Date(Date.now()+60_000).toISOString()}),/BLE_CLIENT_KEY_NOT_PREPARED/);
 const bound=new BleClientHandshake(),boundKey=bound.prepareSessionPublicKey(),other=new BleClientHandshake().prepareSessionPublicKey();await assert.rejects(bound.start({ble_session_id:"s4",signed_session_ticket:ticket("s4",other),protocol_version:BLE_PROTOCOL_VERSION,expires_at:new Date(Date.now()+60_000).toISOString()}),/BLE_TICKET_BINDING/);assert.notEqual(boundKey,other)
});

test("BLE bootstrap reset permits safe session re-issuance",()=>{
 const bootstrap=new WebBleBootstrap(),first=bootstrap.prepareSessionPublicKey();bootstrap.resetSession();const second=bootstrap.prepareSessionPublicKey();assert.notEqual(first,second);assert.equal(bootstrap.canExecuteFiscalCommand,false)
});
