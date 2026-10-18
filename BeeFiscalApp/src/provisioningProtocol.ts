export const PROVISIONING_SERVICE = '7b6f0000-7c6d-4c7a-9e4f-424545464953';
export const PROVISIONING_CONTROL = '7b6f0001-7c6d-4c7a-9e4f-424545464953';
export const PROVISIONING_INFO = '7b6f0005-7c6d-4c7a-9e4f-424545464953';
export const PROVISIONING_STATUS = '7b6f0006-7c6d-4c7a-9e4f-424545464953';
export type InstallProof = {state:'INSTALLED';device_kid:string;binding_sha256:string;package_sha256:string;generation:number;signature:string};
export type ProvisioningLink = {readInfo():Promise<unknown>;readStatus():Promise<unknown>;write(bytes:Uint8Array):Promise<void>;disconnect():Promise<void>};
export type ProvisioningTarget = {deviceID:string;deviceKID:string;bindingID:string;bindingSHA256:string;generation:number};
const hash = /^[a-f0-9]{64}$/;
function object(v:unknown):Record<string,unknown>{if(!v||typeof v!=='object'||Array.isArray(v))throw Error('INVALID_DEVICE_RESPONSE');return v as Record<string,unknown>}
function alive(live:()=>boolean){if(!live())throw Error('PROVISIONING_CONTEXT_CHANGED')}
export function installProof(value:unknown):InstallProof{
 const p=object(value);
 if(p.state!=='INSTALLED'||typeof p.device_kid!=='string'||!p.device_kid||typeof p.binding_sha256!=='string'||!hash.test(p.binding_sha256)||typeof p.package_sha256!=='string'||!hash.test(p.package_sha256)||!Number.isSafeInteger(p.generation)||Number(p.generation)<1||Number(p.generation)>2147483647||typeof p.signature!=='string'||!/^[A-Za-z0-9_-]{86}$/.test(p.signature))throw Error('INVALID_INSTALL_PROOF');
 return {state:'INSTALLED',device_kid:p.device_kid,binding_sha256:p.binding_sha256,package_sha256:p.package_sha256,generation:Number(p.generation),signature:p.signature};
}
export function decodePackage(value:string):Uint8Array{if(!/^[A-Za-z0-9+/]+={0,2}$/.test(value)||value.length>46000)throw Error('INVALID_ENCRYPTED_PACKAGE');const b=Uint8Array.from(atob(value),c=>c.charCodeAt(0));if(b.length<239||b.length>34000||String.fromCharCode(...b.slice(0,4))!=='BFP2')throw Error('INVALID_ENCRYPTED_PACKAGE');return b}
export function provisioningFrames(packet:Uint8Array,sha256:string):Uint8Array[]{
 if(!hash.test(sha256)||packet.length<239||packet.length>34000)throw Error('INVALID_ENCRYPTED_PACKAGE');
 const digest=Uint8Array.from(sha256.match(/../g)!,x=>parseInt(x,16)),frames:Uint8Array[]=[];
 for(let offset=0;offset<packet.length;offset+=121){const part=packet.slice(offset,offset+121),frame=new Uint8Array(61+part.length),v=new DataView(frame.buffer);frame.set([66,70,80,70]);frame.set(digest.slice(0,16),4);v.setUint32(20,packet.length);v.setUint32(24,offset);frame.set(digest,28);frame[60]=offset+part.length===packet.length?1:0;frame.set(part,61);frames.push(frame)}return frames;
}
export async function provisionDevice(options:{target:ProvisioningTarget;settings:{ssid:string;wifiPassword:string;fiscalPassword:string};connect:()=>Promise<ProvisioningLink>;call:(path:string,body:unknown)=>Promise<unknown>;sha256:(data:Uint8Array)=>Promise<string>;live:()=>boolean}):Promise<Record<string,unknown>>{
 const {target,settings,live,call}=options;let link:ProvisioningLink|undefined;const base=`/devices/${encodeURIComponent(target.deviceID)}/provisioning-deliveries`;
 const confirm=async(value:unknown,deliveryID:string,expectedPackage?:string)=>{const proof=installProof(value);if(proof.device_kid!==target.deviceKID||proof.binding_sha256!==target.bindingSHA256||proof.generation!==target.generation||(expectedPackage&&proof.package_sha256!==expectedPackage))throw Error('INSTALLED_BINDING_MISMATCH');alive(live);const result=object(await call(`${base}/${deliveryID}:confirm`,proof));alive(live);if(result.state!=='INSTALLED'||result.binding_id!==target.bindingID||result.binding_sha256!==target.bindingSHA256||result.generation!==target.generation)throw Error('INSTALL_CONFIRMATION_MISMATCH');return result};
 try{alive(live);link=await options.connect();alive(live);const info=object(await link.readInfo());alive(live);if(info.protocol_version!==3||info.device_kid!==target.deviceKID)throw Error('ENROLLED_DEVICE_MISMATCH');const status=object(await link.readStatus());alive(live);
 if(status.state==='INSTALLED')return await confirm(status,'by-proof');
 if(status.state!=='UNPROVISIONED'||info.accepts_install!==true||typeof info.setup_nonce!=='string'||!/^[A-Za-z0-9_-]{43}$/.test(info.setup_nonce))throw Error('DEVICE_NOT_IN_SECURE_SETUP');
 const delivery=object(await call(base,{binding_id:target.bindingID,setup_nonce:info.setup_nonce,ssid:settings.ssid,wifi_password:settings.wifiPassword,fiscal_password:settings.fiscalPassword}));alive(live);
 if(delivery.state!=='ISSUED'||typeof delivery.delivery_id!=='string'||!/^[0-9a-f-]{36}$/i.test(delivery.delivery_id)||delivery.binding_sha256!==target.bindingSHA256||delivery.generation!==target.generation||typeof delivery.package_sha256!=='string'||!hash.test(delivery.package_sha256)||typeof delivery.package_base64!=='string')throw Error('INVALID_DELIVERY');
 const packet=decodePackage(delivery.package_base64);if(await options.sha256(packet)!==delivery.package_sha256)throw Error('ENCRYPTED_PACKAGE_HASH_MISMATCH');alive(live);
 try{for(const frame of provisioningFrames(packet,delivery.package_sha256)){alive(live);await link.write(frame);alive(live)}}catch(error){alive(live);await link.disconnect();link=await options.connect();alive(live);/* Lost final ACK is recovered by a signed read, never by replaying writes. */const recovered=await link.readStatus();return await confirm(recovered,delivery.delivery_id,delivery.package_sha256)}
 const result=await confirm(await link.readStatus(),delivery.delivery_id,delivery.package_sha256);
 alive(live);const reboot=new Uint8Array(36);reboot.set([66,70,82,66]);reboot.set(Uint8Array.from(target.bindingSHA256.match(/../g)!,x=>parseInt(x,16)),4);await link.write(reboot).catch(()=>{/* Installation already proven; reboot ACK can be lost. */});return result;
 }finally{await link?.disconnect().catch(()=>{})}
}
