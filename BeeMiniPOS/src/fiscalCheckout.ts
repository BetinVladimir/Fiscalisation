import {buildDeviceProbeEnvelope,buildOfflineSaleEnvelope,executeCheckout} from "./checkoutTransport.ts";
import type {CheckoutOutcome,OfflineSaleInput} from "./checkoutTransport.ts";
import type {TransportSnapshot} from "./transport.ts";
import type {WebBleBootstrap} from "./webBle.ts";

export type ActiveBleBinding={tenant_id:string;register_id:string;edge_id:string;binding_version:number;expires_at:string};
export type RestCheckout=()=>Promise<{state:string;operation_id?:string;fiscal_reference?:string|null}>;
export async function probeFinalDevice(operationId:string,binding:ActiveBleBinding,ble:Pick<WebBleBootstrap,"sendFiscalCommandAndWait">){const envelope=await buildDeviceProbeEnvelope(operationId,{tenantId:binding.tenant_id,registerId:binding.register_id,deviceId:binding.edge_id,fencingToken:binding.binding_version});const result=await ble.sendFiscalCommandAndWait(envelope,operationId,185,10_000);return result.state==="READY"}

// This adapter is the only place where a POS sale chooses REST or BLE. It is
// deliberately independent of React so Android/iOS adapters can reuse exactly
// the same decision and ambiguous-outcome semantics.
export async function fiscalCheckout(snapshot:TransportSnapshot,operationId:string,sale:OfflineSaleInput,binding:ActiveBleBinding|undefined,restCheckout:RestCheckout,ble:Pick<WebBleBootstrap,"sendFiscalCommandAndWait">):Promise<CheckoutOutcome>{
 const rest=async()=>{const operation=await restCheckout();if(operation.state==="FISCALIZED")return{state:"COMPLETED" as const,fiscalReference:operation.fiscal_reference||operation.operation_id};if(operation.state==="FAILED")return{state:"FAILED" as const,reason:"REST_FISCAL_REJECTED"};if(operation.state==="UNKNOWN")throw new Error("REST_FISCAL_OUTCOME_UNKNOWN");throw new Error("REST_FISCAL_RESPONSE_INVALID")};
 const local=async()=>{if(!binding)throw new Error("BLE_BINDING_REQUIRED");const envelope=await buildOfflineSaleEnvelope(operationId,sale,{tenantId:binding.tenant_id,registerId:binding.register_id,deviceId:binding.edge_id,fencingToken:binding.binding_version});const result=await ble.sendFiscalCommandAndWait(envelope,operationId);if(result.state==="FISCALIZED")return{state:"COMPLETED" as const,fiscalReference:result.fiscal_reference};if(result.state==="REJECTED"||result.state==="BLOCKED")return{state:"FAILED" as const,reason:result.error_code||"BLE_FISCAL_REJECTED"};if(result.state==="FISCAL_RESULT_UNKNOWN")throw new Error("BLE_FISCAL_OUTCOME_UNKNOWN");throw new Error("BLE_FISCAL_RESPONSE_INVALID")};
 return executeCheckout(snapshot,operationId,rest,local)
}
