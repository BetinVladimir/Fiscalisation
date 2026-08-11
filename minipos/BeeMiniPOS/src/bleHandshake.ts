import {decodeStrict,encodeCanonical} from "./cbor.ts";
import {BleFrameSession} from "./bleFrame.ts";
import {aesGcmEncrypt,base64urlDecode,base64urlEncode,hash256,hkdf256,randomBytes,x25519Pair,x25519Shared} from "./portableCrypto.ts";

export const BLE_PROTOCOL_VERSION="2026-08-07";
export type ControlMessage={type:"HELLO"|"CHALLENGE"|"AUTH_PROOF"|"READY"|"ACK"|"NACK";protocol_version:string;session_id:string;counter:number;message_id?:string|null;payload:Record<string,unknown>};
export type HandshakeSession={ble_session_id:string;tenant_id:string;location_id:string;register_id:string;edge_id:string;device_id:string;signed_session_ticket:string;protocol_version:string;expires_at:string};
type Ticket={SessionID:string;TenantID:string;LocationID:string;RegisterID:string;DeviceID:string;FiscalDeviceID:string;ClientPublicKey:string;Scopes:string[]};

const te=new TextEncoder();
function concat(...v:Uint8Array[]){const out=new Uint8Array(v.reduce((n,x)=>n+x.length,0));let p=0;for(const x of v){out.set(x,p);p+=x.length}return out}
function parseTicket(raw:string):Ticket{const wrapper=JSON.parse(new TextDecoder().decode(base64urlDecode(raw))) as {payload:string};return JSON.parse(new TextDecoder().decode(base64urlDecode(wrapper.payload))) as Ticket}
export function handshakeContext(v:Pick<Ticket,"TenantID"|"LocationID"|"RegisterID"|"DeviceID"|"FiscalDeviceID"|"SessionID">){return `${v.TenantID}|${v.LocationID}|${v.RegisterID}|${v.DeviceID}|${v.FiscalDeviceID}|${v.SessionID}|${BLE_PROTOCOL_VERSION}`}

export class BleClientHandshake{
 private state:"NEW"|"HELLO_SENT"|"AUTH_SENT"|"READY"|"FAILED"="NEW";
 private pair?:{privateKey:Uint8Array;publicKey:Uint8Array};private clientNonce?:Uint8Array;private session?:HandshakeSession;private sessionKey?:Uint8Array;private frames?:BleFrameSession;
 get currentState(){return this.state}
 get sessionId(){return this.session?.ble_session_id}
 get canSendComplianceIntent(){return this.state==="READY"&&!!this.sessionKey}
 prepareSessionPublicKey(){if(this.state!=="NEW")throw new Error("BLE_HANDSHAKE_STATE");if(!this.pair)this.pair=x25519Pair();return base64urlEncode(this.pair.publicKey)}
 async start(session:HandshakeSession):Promise<Uint8Array>{
  if(this.state!=="NEW")throw new Error("BLE_HANDSHAKE_STATE");
  if(session.protocol_version!==BLE_PROTOCOL_VERSION)throw new Error("BLE_PROTOCOL_MISMATCH");
  if(new Date(session.expires_at).getTime()<=Date.now())throw new Error("BLE_SESSION_EXPIRED");
  if(!this.pair)throw new Error("BLE_CLIENT_KEY_NOT_PREPARED");const ticket=parseTicket(session.signed_session_ticket);if(ticket.SessionID!==session.ble_session_id||ticket.TenantID!==session.tenant_id||ticket.LocationID!==session.location_id||ticket.RegisterID!==session.register_id||ticket.DeviceID!==session.edge_id||ticket.FiscalDeviceID!==session.device_id||!ticket.Scopes.includes("fiscal.execute")||ticket.ClientPublicKey!==base64urlEncode(this.pair.publicKey))throw new Error("BLE_TICKET_BINDING");
  this.clientNonce=randomBytes(16);this.session=session;this.state="HELLO_SENT";
  return encodeCanonical({type:"HELLO",protocol_version:BLE_PROTOCOL_VERSION,session_id:session.ble_session_id,counter:0,payload:{ticket:session.signed_session_ticket,client_nonce:base64urlEncode(this.clientNonce),ephemeral_public_key:base64urlEncode(this.pair.publicKey)}} satisfies ControlMessage)
 }
 async challenge(raw:Uint8Array):Promise<Uint8Array>{
  if(this.state!=="HELLO_SENT"||!this.pair||!this.clientNonce||!this.session)throw new Error("BLE_HANDSHAKE_STATE");
  const v=decodeStrict<ControlMessage>(raw);if(v.type!=="CHALLENGE"||v.protocol_version!==BLE_PROTOCOL_VERSION||v.session_id!==this.session.ble_session_id||v.counter!==0)throw this.fail("BLE_CHALLENGE_INVALID");
  const edgeNonce=base64urlDecode(String(v.payload.edge_nonce||"")),edgePublic=base64urlDecode(String(v.payload.ephemeral_public_key||""));if(edgeNonce.length<16||edgePublic.length!==32)throw this.fail("BLE_CHALLENGE_INVALID");
  const shared=x25519Shared(this.pair.privateKey,edgePublic),ticketHash=hash256(te.encode(this.session.signed_session_ticket)),salt=hash256(concat(ticketHash,this.clientNonce,edgeNonce)),secret=hkdf256(shared,salt,te.encode("BeeFiscal BLE v1|"+handshakeContext(parseTicket(this.session.signed_session_ticket))));
  const proof=hash256(concat(secret,te.encode(this.session.ble_session_id),this.clientNonce,edgeNonce)),plain=encodeCanonical({proof:base64urlEncode(proof),session_id:this.session.ble_session_id});this.sessionKey=secret.slice();this.frames=await BleFrameSession.client(secret,this.session.ble_session_id);
  const nonce=hash256(te.encode("BeeFiscal BLE auth nonce|"+this.session.ble_session_id)).slice(0,12),aad=encodeCanonical({counter:1,protocol_version:BLE_PROTOCOL_VERSION,session_id:this.session.ble_session_id,type:"AUTH_PROOF"}),ciphertext=aesGcmEncrypt(this.sessionKey,nonce,plain,aad);secret.fill(0);shared.fill(0);this.pair.privateKey.fill(0);this.state="AUTH_SENT";
  return encodeCanonical({type:"AUTH_PROOF",protocol_version:BLE_PROTOCOL_VERSION,session_id:this.session.ble_session_id,counter:1,payload:{ciphertext:base64urlEncode(ciphertext)}} satisfies ControlMessage)
 }
 ready(raw:Uint8Array){if(this.state!=="AUTH_SENT"||!this.session)throw new Error("BLE_HANDSHAKE_STATE");const v=decodeStrict<ControlMessage>(raw);if(v.type!=="READY"||v.session_id!==this.session.ble_session_id||v.protocol_version!==BLE_PROTOCOL_VERSION||v.counter!==1){throw this.fail("BLE_READY_INVALID")}this.state="READY";return v}
 frameSession(){if(!this.canSendComplianceIntent||!this.frames)throw new Error("BLE_NOT_READY");return this.frames}
 private fail(code:string){this.state="FAILED";return new Error(code)}
}
