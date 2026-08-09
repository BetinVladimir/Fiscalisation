import test from "node:test";
import assert from "node:assert/strict";
import {BleClientHandshake,BLE_PROTOCOL_VERSION,handshakeContext} from "./bleHandshake.ts";
import type {ControlMessage} from "./bleHandshake.ts";
import {decodeStrict,encodeCanonical} from "./cbor.ts";
import {WebBleBootstrap} from "./webBle.ts";

function b64(v:Uint8Array){let s="";for(const x of v)s+=String.fromCharCode(x);return btoa(s).replaceAll("+","-").replaceAll("/","_").replaceAll("=","")}
function ticket(sessionId:string,clientPublicKey:string){const payload=b64(new TextEncoder().encode(JSON.stringify({SessionID:sessionId,TenantID:"tenant",LocationID:"location",RegisterID:"register",DeviceID:"edge",FiscalDeviceID:"fiscal-device",ClientPublicKey:clientPublicKey,Scopes:["fiscal.execute"]})));return b64(new TextEncoder().encode(JSON.stringify({payload,signature:"test"})))}
function session(sessionId:string,clientPublicKey:string,expires_at=new Date(Date.now()+60_000).toISOString()){return{ble_session_id:sessionId,tenant_id:"tenant",location_id:"location",register_id:"register",edge_id:"edge",device_id:"fiscal-device",signed_session_ticket:ticket(sessionId,clientPublicKey),protocol_version:BLE_PROTOCOL_VERSION,expires_at}}

test("Go and TypeScript share the complete BLE handshake context vector",()=>assert.equal(handshakeContext({TenantID:"tenant",LocationID:"location",RegisterID:"register",DeviceID:"edge",FiscalDeviceID:"fiscal-device",SessionID:"session"}),"tenant|location|register|edge|fiscal-device|session|2026-08-07"));

test("encrypted client handshake gates fiscal execution until READY",async()=>{
 const h=new BleClientHandshake(),sessionId="session-web-1",clientPublicKey=h.prepareSessionPublicKey(),rawTicket=ticket(sessionId,clientPublicKey);
 const hello=decodeStrict<ControlMessage>(await h.start({...session(sessionId,clientPublicKey),signed_session_ticket:rawTicket}));
 assert.equal(hello.type,"HELLO");assert.equal(h.canExecuteFiscalCommand,false);assert.equal(typeof hello.payload.client_nonce,"string");assert.equal(typeof hello.payload.ephemeral_public_key,"string");
 const edge=await crypto.subtle.generateKey({name:"X25519"} as any,false,["deriveBits"]) as CryptoKeyPair;const edgePublic=new Uint8Array(await crypto.subtle.exportKey("raw",edge.publicKey));
 const challenge=encodeCanonical({type:"CHALLENGE",protocol_version:BLE_PROTOCOL_VERSION,session_id:sessionId,counter:0,payload:{edge_nonce:b64(crypto.getRandomValues(new Uint8Array(16))),ephemeral_public_key:b64(edgePublic),max_chunk:128,window:4}} satisfies ControlMessage);
 const auth=decodeStrict<ControlMessage>(await h.challenge(challenge));assert.equal(auth.type,"AUTH_PROOF");assert.equal(typeof auth.payload.ciphertext,"string");assert.equal(h.canExecuteFiscalCommand,false);
 h.ready(encodeCanonical({type:"READY",protocol_version:BLE_PROTOCOL_VERSION,session_id:sessionId,counter:1,payload:{next_expected_counter:1}} satisfies ControlMessage));assert.equal(h.canExecuteFiscalCommand,true);
});

test("handshake rejects expired session and READY replay",async()=>{
 const expired=new BleClientHandshake(),expiredKey=expired.prepareSessionPublicKey();await assert.rejects(expired.start(session("s",expiredKey,new Date(Date.now()-1).toISOString())),/BLE_SESSION_EXPIRED/);
 const fresh=new BleClientHandshake(),freshKey=fresh.prepareSessionPublicKey();await fresh.start(session("s2",freshKey));
 assert.throws(()=>fresh.ready(encodeCanonical({type:"READY",protocol_version:BLE_PROTOCOL_VERSION,session_id:"s2",counter:1,payload:{}} satisfies ControlMessage)),/BLE_HANDSHAKE_STATE/);
});

test("handshake requires the private key prepared before REST issuance",async()=>{
 const unprepared=new BleClientHandshake();await assert.rejects(unprepared.start(session("s3",b64(new Uint8Array(32)))),/BLE_CLIENT_KEY_NOT_PREPARED/);
 const bound=new BleClientHandshake(),boundKey=bound.prepareSessionPublicKey(),other=new BleClientHandshake().prepareSessionPublicKey();await assert.rejects(bound.start(session("s4",other)),/BLE_TICKET_BINDING/);assert.notEqual(boundKey,other)
});

test("outer BLE identity cannot differ from the signed ticket",async()=>{
 const h=new BleClientHandshake(),key=h.prepareSessionPublicKey();await assert.rejects(h.start({...session("identity",key),device_id:"attacker-device"}),/BLE_TICKET_BINDING/)
 const tenant=new BleClientHandshake(),tenantKey=tenant.prepareSessionPublicKey();await assert.rejects(tenant.start({...session("tenant-identity",tenantKey),tenant_id:"attacker-tenant"}),/BLE_TICKET_BINDING/)
});

test("BLE bootstrap reset permits safe session re-issuance",()=>{
 const bootstrap=new WebBleBootstrap(),first=bootstrap.prepareSessionPublicKey();bootstrap.resetSession();const second=bootstrap.prepareSessionPublicKey();assert.notEqual(first,second);assert.equal(bootstrap.canExecuteFiscalCommand,false)
});
