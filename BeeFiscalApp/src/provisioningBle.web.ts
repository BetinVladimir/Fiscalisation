import {PROVISIONING_SERVICE,PROVISIONING_CONTROL,PROVISIONING_INFO,PROVISIONING_STATUS} from './provisioningProtocol';
import type {ProvisioningLink} from './provisioningProtocol';
export async function connectProvisioningDevice():Promise<ProvisioningLink>{
 const bluetooth=(navigator as any).bluetooth;if(!bluetooth||!globalThis.isSecureContext)throw Error('WEB_BLUETOOTH_REQUIRES_CHROME_SECURE_CONTEXT');
 const device=await bluetooth.requestDevice({filters:[{services:[PROVISIONING_SERVICE]}]});
 try{const server=await device.gatt.connect(),service=await server.getPrimaryService(PROVISIONING_SERVICE),control=await service.getCharacteristic(PROVISIONING_CONTROL),info=await service.getCharacteristic(PROVISIONING_INFO),status=await service.getCharacteristic(PROVISIONING_STATUS);
 const read=async(c:any)=>{const v:DataView=await c.readValue();return JSON.parse(new TextDecoder().decode(new Uint8Array(v.buffer,v.byteOffset,v.byteLength)))};
 return {readInfo:()=>read(info),readStatus:()=>read(status),write:async(bytes)=>{await control.writeValueWithResponse(bytes)},disconnect:async()=>device.gatt?.disconnect()};
 }catch(error){device.gatt?.disconnect();throw error}
}
