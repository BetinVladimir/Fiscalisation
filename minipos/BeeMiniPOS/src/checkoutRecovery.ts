import type {components} from '../../../contracts/generated/openapi-runtime-v1';
import type {DurableCheckout} from './checkoutJournal.ts';
import type {SourceOrder} from './sourceOrders.ts';
import {confirmCloudCheckout,validateCloudOperation,type CloudCheckoutOperation,type CloudCheckoutExpected} from './cloudCheckoutConfirmation.ts';
export type RecoveryPermission=components['schemas']['MiniPosOrderRecoveryPermission'];
export type RecoveryScope={operatorCode:string;fiscalRegisterID:string;shiftID?:string;registerID?:string};
const nonempty=(v:unknown):v is string=>typeof v==='string'&&v.trim().length>0;
/** Discover an owner-authorized source graph, then request its current READ
 * capability. No write, prepare, dispatch or payment retry is reachable here. */
export async function recoverCheckoutReadOnly(options:{read:<T>(path:string)=>Promise<T>;durable:DurableCheckout;scope:RecoveryScope;journal:{remove(id:string):Promise<void>};live:()=>boolean;onPermission?:(p:RecoveryPermission)=>void}){
 const {read,durable:d,scope:s,journal,live}=options;
 const alive=()=>{if(!live())throw Error('CHECKOUT_AUTHORITY_CHANGED')};
 const sourceID=d.source_order_id;
 if(!Number.isSafeInteger(d.sale_version)||d.sale_version<1)throw Error('CHECKOUT_RECOVERY_VERSION_REQUIRED');
 if(!nonempty(sourceID)||sourceID!==d.sale_id)throw Error('LEGACY_SOURCE_RECOVERY_REQUIRED');
 if(!nonempty(s.operatorCode)||!nonempty(s.fiscalRegisterID))throw Error('CHECKOUT_RECOVERY_CONTEXT_REQUIRED');
 const frozen=d.source_context,signed=d.signed_intent;
 if((frozen&&frozen.operator_code!==s.operatorCode)||(nonempty(signed?.register_id)&&signed.register_id!==s.fiscalRegisterID))throw Error('CHECKOUT_RECOVERY_CONTEXT_MISMATCH');
 alive();
 // Closed source shifts remain recoverable: active UI shift IDs are not invented
 // as their parent. The public endpoint enforces the current actor/session.
 const source=await read<SourceOrder>(`/orders/${encodeURIComponent(sourceID)}`);alive();
 if(!source||source.id!==sourceID||!nonempty(source.shift_id)||!nonempty(source.register_id)||!nonempty(source.external_id)||source.operator_code!==s.operatorCode||!Number.isSafeInteger(source.version)||source.version<d.sale_version||(s.shiftID&&source.shift_id!==s.shiftID)||(s.registerID&&source.register_id!==s.registerID)||(source.checkout_confirmation?.root_register_id&&source.checkout_confirmation.root_register_id!==s.fiscalRegisterID))throw Error('CHECKOUT_RECOVERY_SOURCE_MISMATCH');
 if(frozen&&(source.shift_id!==frozen.shift_id||source.register_id!==frozen.register_id||source.operator_code!==frozen.operator_code||source.external_id!==frozen.external_id))throw Error('CHECKOUT_RECOVERY_SOURCE_CONTEXT_MISMATCH');
 const permission=await read<RecoveryPermission>(`/orders/${encodeURIComponent(sourceID)}/recovery`);alive();
 if(!permission||permission.order_id!==sourceID||permission.shift_id!==source.shift_id||permission.register_id!==source.register_id||permission.operator_code!==source.operator_code||permission.external_id!==source.external_id||!Number.isSafeInteger(permission.order_version)||permission.order_version<source.version||!Array.isArray(permission.allowed_actions)||permission.allowed_actions.some(a=>a!=='READ'))throw Error('CHECKOUT_RECOVERY_PERMISSION_MISMATCH');
 if(!permission.allowed_actions.includes('READ')){options.onPermission?.(permission);return{kind:'DENIED' as const,permission};}
 if(!['FISCAL_PENDING','UNKNOWN','COMPLETED','REVERSED','FAILED'].includes(permission.state)||permission.fiscal_register_id!==s.fiscalRegisterID||[permission.client_operation_id,permission.receipt_session_id,permission.fiscal_register_id].some(v=>!nonempty(v)))throw Error('CHECKOUT_RECOVERY_ASSOCIATION_UNAVAILABLE');
 if(signed&&(permission.client_operation_id!==d.plan.client_operation_id||permission.receipt_session_id!==d.plan.receipt_session_id||permission.fiscal_register_id!==signed.register_id||(nonempty(signed.server_sale_id)&&nonempty(permission.fiscal_sale_id)&&permission.fiscal_sale_id!==signed.server_sale_id)||permission.external_id!==signed.client_sale_surrogate_id))throw Error('CHECKOUT_RECOVERY_INTENT_MISMATCH');
 let fiscalSaleID=permission.fiscal_sale_id;
 if(!nonempty(fiscalSaleID)){
  const operation=await read<CloudCheckoutOperation>(`/orders/${encodeURIComponent(sourceID)}/operations/${encodeURIComponent(permission.client_operation_id)}`);alive();
  if(!nonempty(operation?.sale_id))throw Error('CHECKOUT_RECOVERY_ASSOCIATION_UNAVAILABLE');
  validateCloudOperation(operation,{operationID:permission.client_operation_id,receiptSessionID:permission.receipt_session_id,fiscalSaleID:operation.sale_id,fiscalRegisterID:permission.fiscal_register_id});
  fiscalSaleID=operation.sale_id;
  if(nonempty(signed?.server_sale_id)&&fiscalSaleID!==signed.server_sale_id)throw Error('CHECKOUT_RECOVERY_INTENT_MISMATCH');
 }
 const expected:CloudCheckoutExpected={sourceOrderID:sourceID,shiftID:source.shift_id,sourceRegisterID:source.register_id,operatorCode:source.operator_code,externalID:source.external_id,sourceVersion:permission.order_version,operationID:permission.client_operation_id,receiptSessionID:permission.receipt_session_id,fiscalSaleID,fiscalRegisterID:permission.fiscal_register_id,total:d.plan.expected_total,payments:d.plan.payments};
 alive();options.onPermission?.(permission);
 const confirmed=await confirmCloudCheckout({read,expected,journal,live,observed:d.cloud_observation});alive();
 return{...confirmed,permission};
}
