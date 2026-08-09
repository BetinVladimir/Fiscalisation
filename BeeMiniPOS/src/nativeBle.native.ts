import {PermissionsAndroid,Platform} from "react-native";
import {BleManager} from "react-native-ble-plx";
import type {BleError,Characteristic} from "react-native-ble-plx";
import {decodeStrict,encodeCanonical} from "./cbor.ts";
import {BleClientHandshake} from "./bleHandshake.ts";
import type {ControlMessage} from "./bleHandshake.ts";
import {chunkPlaintext} from "./bleFrame.ts";
import {ackControl,BleMessageAssembler,nackControl} from "./bleFlow.ts";
import {BleResultWaiters} from "./bleResult.ts";
import type {BleFiscalResult} from "./bleResult.ts";
import type {BleSessionPackage} from "./webBle.ts";
import type {NativeBleBootstrapContract} from "./nativeBle.ts";
import {base64urlDecode,base64urlEncode} from "./portableCrypto.ts";
import {authorizedAdvertisingIdentity,matchesAdvertisingIdentity} from "./bleAdvertising.ts";

export async function requestNativeBlePermissions(){if(Platform.OS!=="android")return true;const api=Number(Platform.Version);if(api>=31){const result=await PermissionsAndroid.requestMultiple([PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN,PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT]);return result[PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN]===PermissionsAndroid.RESULTS.GRANTED&&result[PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT]===PermissionsAndroid.RESULTS.GRANTED}return(await PermissionsAndroid.request(PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION))===PermissionsAndroid.RESULTS.GRANTED}

class NativeBleBootstrap implements NativeBleBootstrapContract{
 private manager=new BleManager();private deviceId?:string;private session?:BleSessionPackage;private handshake=new BleClientHandshake();private assembler=new BleMessageAssembler();private results=new BleResultWaiters();
 get canExecuteFiscalCommand(){return this.handshake.canExecuteFiscalCommand}
 prepareSessionPublicKey(){return this.handshake.prepareSessionPublicKey()}
 resetSession(){this.results.cancelAll();if(this.deviceId)void this.manager.cancelDeviceConnection(this.deviceId).catch(()=>{});this.deviceId=undefined;this.session=undefined;this.handshake=new BleClientHandshake();this.assembler=new BleMessageAssembler();this.results=new BleResultWaiters()}
 async connectFromUserGesture(session:BleSessionPackage,onEvent:(raw:Uint8Array)=>void){if(new Date(session.expires_at).getTime()<=Date.now())throw new Error("BLE_SESSION_EXPIRED");const hello=await this.handshake.start(session);const advertisingIdentity=authorizedAdvertisingIdentity(session.advertising_identity,session.edge_id);const device=await new Promise<any>((resolve,reject)=>{let done=false;const timer=setTimeout(()=>{if(done)return;done=true;this.manager.stopDeviceScan();reject(new Error("BLE_SCAN_TIMEOUT"))},10_000);this.manager.startDeviceScan([session.service_uuid],null,(error,found)=>{if(done)return;if(error){done=true;clearTimeout(timer);this.manager.stopDeviceScan();reject(error);return}if(found&&matchesAdvertisingIdentity(found,advertisingIdentity)){done=true;clearTimeout(timer);this.manager.stopDeviceScan();resolve(found)}})});const connected=await device.connect();await connected.discoverAllServicesAndCharacteristics();if(Platform.OS==="android")await connected.requestMTU(185);this.deviceId=connected.id;this.session=session;
  let resolveReady!:()=>void,rejectReady!:(e:unknown)=>void;const ready=new Promise<void>((resolve,reject)=>{resolveReady=resolve;rejectReady=reject});connected.monitorCharacteristicForService(session.service_uuid,session.event_characteristic_uuid,(error:BleError|null,characteristic:Characteristic|null)=>{if(error){rejectReady(error);return}if(!characteristic?.value)return;void this.handleEvent(fromStandardBase64(characteristic.value),onEvent,resolveReady,rejectReady)});await this.write(controlUUID(session.service_uuid,"0001"),hello);return{state:"HELLO_SENT" as const,ready}
 }
 async sendFiscalCommandAndWait(envelope:unknown,messageId:string,attMtu=185,timeoutMs=30_000):Promise<BleFiscalResult>{if(!this.handshake.canExecuteFiscalCommand)throw new Error("BLE_NOT_READY");const result=this.results.wait(messageId,timeoutMs);try{const chunks=chunkPlaintext(encodeCanonical(envelope),attMtu);for(let i=0;i<chunks.length;i++)await this.write(this.session!.command_characteristic_uuid,await this.handshake.frameSession().seal(messageId,i,chunks.length,0,chunks[i]))}catch(e){this.results.reject(messageId,e)}return result}
 private async handleEvent(raw:Uint8Array,onEvent:(raw:Uint8Array)=>void,resolve:()=>void,reject:(e:unknown)=>void){try{if(this.handshake.currentState==="HELLO_SENT"){const v=decodeStrict<ControlMessage>(raw);if(v.type!=="CHALLENGE")throw new Error("BLE_CHALLENGE_EXPECTED");await this.write(controlUUID(this.session!.service_uuid,"0001"),await this.handshake.challenge(raw));return}if(this.handshake.currentState==="AUTH_SENT"){this.handshake.ready(raw);resolve();return}const frame=await this.handshake.frameSession().open(raw);let flow:ReturnType<BleMessageAssembler["accept"]>;try{flow=this.assembler.accept(frame)}catch(e){await this.sendFlow(frame.messageId,frame.counter,encodeCanonical(nackControl(this.handshake.sessionId!,frame.counter,frame.messageId,e instanceof Error&&e.message==="BLE_BUSY"?"BUSY":"BAD_CHUNK")));throw e}await this.sendFlow(frame.messageId,frame.counter,encodeCanonical(ackControl(this.handshake.sessionId!,frame.counter,flow)));if(flow.complete){this.results.accept(frame.messageId,flow.complete);onEvent(flow.complete)}}catch(e){reject(e)}}
 private async sendFlow(messageId:string,_counter:bigint,payload:Uint8Array){await this.write(controlUUID(this.session!.service_uuid,"0004"),await this.handshake.frameSession().seal(messageId,0,1,2,payload))}
 private async write(characteristic:string,raw:Uint8Array){if(!this.deviceId||!this.session)throw new Error("BLE_NOT_CONNECTED");await this.manager.writeCharacteristicWithResponseForDevice(this.deviceId,this.session.service_uuid,characteristic,toStandardBase64(raw))}
}
export function createNativeBleBootstrap(){return new NativeBleBootstrap()}
function controlUUID(service:string,suffix:"0001"|"0004"){const normalized=service.toLowerCase();if(!normalized.startsWith("7b6f0000-"))throw new Error("BLE_SERVICE_UUID_INVALID");return"7b6f"+suffix+normalized.slice(8)}
function toStandardBase64(v:Uint8Array){const raw=base64urlEncode(v).replaceAll("-","+").replaceAll("_","/");return raw+"=".repeat((4-raw.length%4)%4)}
function fromStandardBase64(v:string){return base64urlDecode(v.replaceAll("+","-").replaceAll("/","_").replaceAll("=",""))}
