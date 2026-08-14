import {decodeStrict,encodeCanonical} from "./cbor.ts";
import {BleResultWaiters} from "./bleResult.ts";
import type {BleFiscalResult} from "./bleResult.ts";
import {base64urlEncode,randomBytes} from "./portableCrypto.ts";
import {authorizedAdvertisingIdentity,matchesAdvertisingIdentity} from "./bleAdvertising.ts";
import {FrameAssembler,frameMessage} from "./bleFrames.ts";
export type BleSessionPackage={ble_session_id:string;tenant_id:string;location_id:string;register_id:string;edge_id:string;device_id:string;service_uuid:string;command_characteristic_uuid:string;event_characteristic_uuid:string;advertising_identity:string;signed_session_ticket:string;expires_at:string;protocol_version:string;security_mode:"OPEN_MVP"|"X25519_AES_GCM"};
type Characteristic={writeValueWithResponse?(v:BufferSource):Promise<void>;writeValue?(v:BufferSource):Promise<void>;startNotifications():Promise<Characteristic>;addEventListener(type:string,listener:(e:any)=>void):void};
type Device={name?:string|null;gatt?:{connect():Promise<{getPrimaryService(uuid:string):Promise<{getCharacteristic(uuid:string):Promise<Characteristic>}>}>;disconnect?():void}};
export function webBluetoothSupported(){return typeof navigator!=="undefined"&&!!(navigator as any).bluetooth&&globalThis.isSecureContext===true}
export class WebBleBootstrap{
 private device?:Device;private command?:Characteristic;private event?:Characteristic;private ready=false;private results=new BleResultWaiters();private frames=new FrameAssembler();
 get canSendComplianceIntent(){return this.ready}
 prepareSessionPublicKey(){return base64urlEncode(randomBytes(32))}
 resetSession(){this.results.cancelAll();this.device?.gatt?.disconnect?.();this.device=undefined;this.command=undefined;this.event=undefined;this.ready=false;this.results=new BleResultWaiters()}
 async connectFromUserGesture(session:BleSessionPackage,onEvent:(raw:Uint8Array)=>void){
  if(!webBluetoothSupported())throw new Error("WEB_BLUETOOTH_UNAVAILABLE");if(session.security_mode!=="OPEN_MVP")throw new Error("BLE_SECURITY_MODE_UNSUPPORTED");if(new Date(session.expires_at).getTime()<=Date.now())throw new Error("BLE_ROUTE_PACKAGE_EXPIRED");
  const advertisingIdentity=authorizedAdvertisingIdentity(session.advertising_identity,session.edge_id);this.device=await (navigator as any).bluetooth.requestDevice({filters:[{services:[session.service_uuid]}]}) as Device;if(!matchesAdvertisingIdentity(this.device,advertisingIdentity)){this.device.gatt?.disconnect?.();this.device=undefined;throw new Error("BLE_ADVERTISING_IDENTITY_MISMATCH")}
  const server=await this.device.gatt?.connect();if(!server)throw new Error("BLE_GATT_UNAVAILABLE");const service=await server.getPrimaryService(session.service_uuid);this.command=await service.getCharacteristic(session.command_characteristic_uuid);this.event=await service.getCharacteristic(session.event_characteristic_uuid);await this.event.startNotifications();this.event.addEventListener("characteristicvaluechanged",e=>{const value=e.target?.value as DataView|undefined;if(!value)return;const frame=new Uint8Array(value.buffer,value.byteOffset,value.byteLength);void this.frames.accept(frame).then(done=>{if(!done)return;this.results.accept(done.id,done.payload);onEvent(done.payload)}).catch(()=>{})});this.ready=true;return{state:"OPEN_READY" as const,ready:Promise.resolve()}
 }
 async sendComplianceIntent(intent:unknown,_messageId:string,_attMtu=185){if(!this.ready)throw new Error("BLE_NOT_READY");const frames=await frameMessage(encodeCanonical(intent),_messageId,_attMtu);for(const frame of frames)await this.write(this.command,frame);return{messageId:_messageId,chunks:frames.length}}
 async sendComplianceIntentAndWait(intent:unknown,messageId:string,_attMtu=185,timeoutMs=30_000){const result=this.results.wait(messageId,timeoutMs);try{await this.sendComplianceIntent(intent,messageId)}catch(e){this.results.reject(messageId,e)}return result}
 private async write(characteristic:Characteristic|undefined,value:Uint8Array){if(!characteristic)throw new Error("BLE_NOT_CONNECTED");const data=value as unknown as BufferSource;if(characteristic.writeValueWithResponse)await characteristic.writeValueWithResponse(data);else if(characteristic.writeValue)await characteristic.writeValue(data);else throw new Error("BLE_WRITE_UNSUPPORTED")}
}
function operationId(raw:Uint8Array){try{const value=decodeStrict<Partial<BleFiscalResult>>(raw);return typeof value.operation_id==="string"?value.operation_id:""}catch{return ""}}
