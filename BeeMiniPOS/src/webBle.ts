import {decodeStrict} from "./cbor.ts";
import {BleClientHandshake} from "./bleHandshake.ts";
import type {ControlMessage} from "./bleHandshake.ts";
import {encodeCanonical} from "./cbor.ts";
import {chunkPlaintext} from "./bleFrame.ts";
import {ackControl,BleMessageAssembler,nackControl} from "./bleFlow.ts";
import {BleResultWaiters} from "./bleResult.ts";
import type {BleFiscalResult} from "./bleResult.ts";
export type BleSessionPackage={ble_session_id:string;service_uuid:string;command_characteristic_uuid:string;event_characteristic_uuid:string;advertising_identity:string;signed_session_ticket:string;expires_at:string;protocol_version:string};
type Characteristic={writeValueWithResponse?(v:BufferSource):Promise<void>;writeValue?(v:BufferSource):Promise<void>;startNotifications():Promise<Characteristic>;addEventListener(type:string,listener:(e:any)=>void):void};
type Device={gatt?:{connect():Promise<{getPrimaryService(uuid:string):Promise<{getCharacteristic(uuid:string):Promise<Characteristic>}>}>;disconnect?():void}};
export function webBluetoothSupported(){return typeof navigator!=="undefined"&&!!(navigator as any).bluetooth&&globalThis.isSecureContext===true}
export class WebBleBootstrap{
 private device?:Device;private control?:Characteristic;private command?:Characteristic;private event?:Characteristic;private flow?:Characteristic;private handshake=new BleClientHandshake();private assembler=new BleMessageAssembler();private results=new BleResultWaiters();
 get canExecuteFiscalCommand(){return this.handshake.canExecuteFiscalCommand}
 prepareSessionPublicKey(){return this.handshake.prepareSessionPublicKey()}
 resetSession(){this.results.cancelAll();this.device?.gatt?.disconnect?.();this.device=undefined;this.control=undefined;this.command=undefined;this.event=undefined;this.flow=undefined;this.handshake=new BleClientHandshake();this.assembler=new BleMessageAssembler();this.results=new BleResultWaiters()}
 async connectFromUserGesture(session:BleSessionPackage,onEvent:(raw:Uint8Array)=>void){
  if(!webBluetoothSupported())throw new Error("WEB_BLUETOOTH_UNAVAILABLE");
  if(new Date(session.expires_at).getTime()<=Date.now())throw new Error("BLE_SESSION_EXPIRED");
  this.device=await (navigator as any).bluetooth.requestDevice({filters:[{services:[session.service_uuid]}]}) as Device;
  const server=await this.device.gatt?.connect();if(!server)throw new Error("BLE_GATT_UNAVAILABLE");const service=await server.getPrimaryService(session.service_uuid);this.control=await service.getCharacteristic(characteristicUUID(session.service_uuid,"0001"));this.command=await service.getCharacteristic(session.command_characteristic_uuid);this.event=await service.getCharacteristic(session.event_characteristic_uuid);this.flow=await service.getCharacteristic(characteristicUUID(session.service_uuid,"0004"));await this.event.startNotifications();
  let resolveReady!:()=>void,rejectReady!:(e:unknown)=>void;const ready=new Promise<void>((resolve,reject)=>{resolveReady=resolve;rejectReady=reject});
  this.event.addEventListener("characteristicvaluechanged",e=>{const value=e.target?.value as DataView|undefined;if(!value)return;const raw=new Uint8Array(value.buffer,value.byteOffset,value.byteLength);void this.handleHandshakeEvent(raw,onEvent,resolveReady,rejectReady)});
  const hello=await this.handshake.start(session);await this.write(this.control,hello);return {state:"HELLO_SENT" as const,ready};
 }
 async sendFiscalCommand(envelope:unknown,messageId:string,attMtu=185){if(!this.handshake.canExecuteFiscalCommand)throw new Error("BLE_NOT_READY");const payload=encodeCanonical(envelope),chunks=chunkPlaintext(payload,attMtu);for(let i=0;i<chunks.length;i++)await this.write(this.command,await this.handshake.frameSession().seal(messageId,i,chunks.length,0,chunks[i]));return {messageId,chunks:chunks.length}}
 async sendFiscalCommandAndWait(envelope:unknown,messageId:string,attMtu=185,timeoutMs=30_000):Promise<BleFiscalResult>{const result=this.results.wait(messageId,timeoutMs);try{await this.sendFiscalCommand(envelope,messageId,attMtu)}catch(e){this.results.reject(messageId,e)}return result}
 private async handleHandshakeEvent(raw:Uint8Array,onEvent:(raw:Uint8Array)=>void,resolve:()=>void,reject:(e:unknown)=>void){try{if(this.handshake.currentState==="HELLO_SENT"){const v=decodeStrict<ControlMessage>(raw);if(v.type!=="CHALLENGE")throw new Error("BLE_CHALLENGE_EXPECTED");await this.write(this.control,await this.handshake.challenge(raw));return}if(this.handshake.currentState==="AUTH_SENT"){this.handshake.ready(raw);resolve();return}if(this.handshake.canExecuteFiscalCommand){const frame=await this.handshake.frameSession().open(raw);let flow:ReturnType<BleMessageAssembler["accept"]>;try{flow=this.assembler.accept(frame)}catch(e){const reason=e instanceof Error&&e.message==="BLE_BUSY"?"BUSY":"BAD_CHUNK";await this.sendFlowNack(frame.messageId,frame.counter,reason);throw e}await this.sendFlowAck(frame.messageId,frame.counter,flow);if(flow.complete){this.results.accept(frame.messageId,flow.complete);onEvent(flow.complete)}}}catch(e){reject(e)}}
 private async sendFlowAck(messageId:string,counter:bigint,state:ReturnType<BleMessageAssembler["accept"]>){const sessionId=this.handshake.sessionId;if(!sessionId)throw new Error("BLE_NOT_READY");const payload=encodeCanonical(ackControl(sessionId,counter,state)),frame=await this.handshake.frameSession().seal(messageId,0,1,2,payload);await this.write(this.flow,frame)}
 private async sendFlowNack(messageId:string,counter:bigint,reason:"BUSY"|"BAD_CHUNK"){const sessionId=this.handshake.sessionId;if(!sessionId)throw new Error("BLE_NOT_READY");const payload=encodeCanonical(nackControl(sessionId,counter,messageId,reason)),frame=await this.handshake.frameSession().seal(messageId,0,1,2,payload);await this.write(this.flow,frame)}
 private async write(c:Characteristic|undefined,v:Uint8Array){if(!c)throw new Error("BLE_NOT_CONNECTED");const data=v as unknown as BufferSource;if(c.writeValueWithResponse)await c.writeValueWithResponse(data);else if(c.writeValue)await c.writeValue(data);else throw new Error("BLE_WRITE_UNSUPPORTED")}
}
function characteristicUUID(serviceUUID:string,suffix:"0001"|"0004"){const normalized=serviceUUID.toLowerCase();if(!normalized.startsWith("7b6f0000-"))throw new Error("BLE_SERVICE_UUID_INVALID");return "7b6f"+suffix+normalized.slice(8)}
