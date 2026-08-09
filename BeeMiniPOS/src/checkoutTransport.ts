import {encodeCanonical} from "./cbor.ts";
import {chooseTransport} from "./transport.ts";
import type {TransportSnapshot} from "./transport.ts";
import {hash256} from "./portableCrypto.ts";

export type CheckoutOutcome={operationId:string;route:"REST"|"BLE"|"NONE";state:"COMPLETED"|"ACCEPTED"|"FAILED"|"UNKNOWN"|"BLOCKED";fiscalReference?:string;reason?:string;requiredAction?:"RECONCILE"};
export type RouteExecutor=(operationId:string)=>Promise<{state:"COMPLETED"|"ACCEPTED"|"FAILED";fiscalReference?:string;reason?:string}>;

// Route is selected before the first byte of the fiscal command is sent. Once
// an executor is called, any transport error is UNKNOWN and must reconcile;
// falling through to the other executor would risk a second fiscal receipt.
export async function executeCheckout(snapshot:TransportSnapshot,operationId:string,rest:RouteExecutor,ble:RouteExecutor,now=new Date()):Promise<CheckoutOutcome>{
 if(!operationId)throw new Error("OPERATION_ID_REQUIRED");const decision=chooseTransport(snapshot,now);if(decision.transport==="BLOCK")return{operationId,route:"NONE",state:"BLOCKED",reason:decision.reason};const executor=decision.transport==="REST"?rest:ble;try{const result=await executor(operationId);return{operationId,route:decision.transport,state:result.state,fiscalReference:result.fiscalReference,reason:result.reason}}catch{return{operationId,route:decision.transport,state:"UNKNOWN",reason:"OUTCOME_AMBIGUOUS",requiredAction:"RECONCILE"}}
}

export type OfflineSaleInput={external_id:string;operator_id:string;currency:"EUR";items:Array<Record<string,unknown>>;payments:Array<Record<string,unknown>>;metadata:Record<string,unknown>};
export type OfflineBinding={tenantId:string;locationId:string;registerId:string;deviceId:string;fencingToken:number};
export async function buildOfflineSaleEnvelope(operationId:string,input:OfflineSaleInput,binding:OfflineBinding,now=new Date()){
 if(!operationId||!input.external_id||input.operator_id.length!==4||input.currency!=="EUR"||!input.items.length||!input.payments.length||!binding.tenantId||!binding.locationId||!binding.registerId||!binding.deviceId||binding.fencingToken<1)throw new Error("OFFLINE_SALE_INVALID");const payload={currency:input.currency,external_id:input.external_id,location_id:binding.locationId,operator_id:input.operator_id,items:input.items,payments:input.payments,metadata:input.metadata};const hash=hash256(encodeCanonical(payload));return{operation_id:operationId,tenant_id:binding.tenantId,register_id:binding.registerId,device_id:binding.deviceId,fencing_token:binding.fencingToken,command_type:"FISCAL_SALE",issued_at:now.toISOString(),expires_at:new Date(now.getTime()+60_000).toISOString(),payload,payload_sha256:Array.from(hash,x=>x.toString(16).padStart(2,"0")).join("")}
}
export async function buildDeviceProbeEnvelope(operationId:string,binding:OfflineBinding,now=new Date()){
 if(!operationId||!binding.tenantId||!binding.locationId||!binding.registerId||!binding.deviceId||binding.fencingToken<1)throw new Error("DEVICE_PROBE_INVALID");const payload:Record<string,unknown>={},hash=hash256(encodeCanonical(payload));return{operation_id:operationId,tenant_id:binding.tenantId,register_id:binding.registerId,device_id:binding.deviceId,fencing_token:binding.fencingToken,command_type:"DEVICE_PROBE",issued_at:now.toISOString(),expires_at:new Date(now.getTime()+15_000).toISOString(),payload,payload_sha256:Array.from(hash,x=>x.toString(16).padStart(2,"0")).join("")}
}
