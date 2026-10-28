import {canonical} from './goCanonical.ts';
import {payloadDigest} from './nativeExecutionAuthority.ts';
import type {SourceOrder} from './sourceOrders.ts';
export type CloudCheckoutOperation={operation_id:string;client_operation_id:string;receipt_session_id:string;sale_id:string;register_id:string;type:string;state:string;version:number;fiscal_reference?:string|null;allowed_actions?:string[]};
export type CloudCheckoutExpected={sourceOrderID:string;shiftID:string;sourceRegisterID:string;operatorCode:string;externalID:string;sourceVersion:number;operationID:string;receiptSessionID:string;fiscalSaleID:string;fiscalRegisterID:string;total:{amount:string;currency:'EUR'};payments:unknown[]};
/** Match source normalizedOrderPayments; ignore terminal/card/provider metadata. */
export function preparedPaymentsDigest(payments:unknown[]):string {
 const normalized=payments.map(value=>{
  const p=value as Record<string,unknown>, amount=p?.amount as Record<string,unknown>;
  if(!p||typeof p.type!=='string'||!amount||typeof amount.amount!=='string'||typeof amount.currency!=='string')throw Error('CHECKOUT_PAYMENT_PROOF_INVALID');
  return {type:p.type,amount:{amount:amount.amount,currency:amount.currency},...(typeof p.payment_id==='string'&&p.payment_id?{payment_id:p.payment_id}:{}),...(typeof p.terminal_policy==='string'&&p.terminal_policy?{terminal_policy:p.terminal_policy}:{})};
 });
 return payloadDigest(normalized);
}
const alive=(live:()=>boolean)=>{if(!live())throw Error('CHECKOUT_AUTHORITY_CHANGED');};
const required=(value:unknown):value is string=>typeof value==='string'&&value.trim().length>0;
export function validateCloudOperation(operation:CloudCheckoutOperation,expected:Pick<CloudCheckoutExpected,'operationID'|'receiptSessionID'|'fiscalSaleID'|'fiscalRegisterID'>){
 if(!operation||operation.operation_id!==expected.operationID||operation.client_operation_id!==expected.operationID||operation.receipt_session_id!==expected.receiptSessionID||operation.sale_id!==expected.fiscalSaleID||operation.register_id!==expected.fiscalRegisterID||!['SALE_FINALIZE','FISCAL_SALE'].includes(operation.type)||!Number.isSafeInteger(operation.version)||operation.version<1||!['RECEIVED','VALIDATED','ROUTED','ACCEPTED_BY_EDGE','EXECUTING','FISCALIZED','FAILED','UNKNOWN','RECONCILING','SYNCED','CANCELLED'].includes(operation.state))throw Error('FISCAL_OPERATION_IDENTITY_MISMATCH');
}
/** Keep only the bound receipt proof, not arbitrary upstream operation metadata. */
export function cloudReceiptObservation(operation:CloudCheckoutOperation,expected:Pick<CloudCheckoutExpected,'operationID'|'receiptSessionID'|'fiscalSaleID'|'fiscalRegisterID'>):CloudCheckoutOperation{
 validateCloudOperation(operation,expected);
 if(operation.state!=='FISCALIZED'||!required(operation.fiscal_reference))throw Error('CLOUD_CHECKOUT_OBSERVATION_INVALID');
 const {operation_id,client_operation_id,receipt_session_id,sale_id,register_id,type,state,version,fiscal_reference}=operation;
 return {operation_id,client_operation_id,receipt_session_id,sale_id,register_id,type,state,version,fiscal_reference};
}
/** Two actor-scoped source GETs are required before terminal journal cleanup.
 * This function never dispatches, retries payment, or accepts a BLE reply as proof. */
export async function confirmCloudCheckout(options:{read:<T>(path:string)=>Promise<T>;expected:CloudCheckoutExpected;journal:{remove(id:string):Promise<void>};live:()=>boolean;observed?:CloudCheckoutOperation}){
 const {read,expected:e,journal,live,observed}=options;
 if(observed){validateCloudOperation(observed,e);if(observed.state==='FISCALIZED'&&!required(observed.fiscal_reference))throw Error('CLOUD_CHECKOUT_OBSERVATION_INVALID');}
 if([e.sourceOrderID,e.shiftID,e.sourceRegisterID,e.operatorCode,e.externalID,e.operationID,e.receiptSessionID,e.fiscalSaleID,e.fiscalRegisterID].some(v=>!required(v)))throw Error('CHECKOUT_CONFIRMATION_IDENTITY_REQUIRED');
 alive(live);
 const operation=await read<CloudCheckoutOperation>(`/orders/${encodeURIComponent(e.sourceOrderID)}/operations/${encodeURIComponent(e.operationID)}`);alive(live);validateCloudOperation(operation,e);
 if(observed?.state==='FISCALIZED'&&(['FAILED','CANCELLED'].includes(operation.state)||(operation.state==='FISCALIZED'&&operation.fiscal_reference!==observed.fiscal_reference)))throw Error('CLOUD_CHECKOUT_CONTRADICTORY_RESULT');
 const terminal=(state:string)=>state==='FISCALIZED'?'FISCALIZED':['FAILED','CANCELLED'].includes(state)?'FAILED':undefined;
 if(observed&&observed.version===operation.version&&terminal(operation.state)&&terminal(observed.state)!==terminal(operation.state))throw Error('CLOUD_CHECKOUT_CONTRADICTORY_RESULT');
 const order=await read<SourceOrder>(`/orders/${encodeURIComponent(e.sourceOrderID)}`);alive(live);
 if(!order||order.id!==e.sourceOrderID||order.shift_id!==e.shiftID||order.register_id!==e.sourceRegisterID||order.operator_code!==e.operatorCode||order.external_id!==e.externalID||order.fiscal_sale_id!==e.fiscalSaleID||order.fiscal_operation_id!==e.operationID||order.checkout_operation_id!==e.operationID||order.receipt_session_id!==e.receiptSessionID||order.checkout_confirmation?.version!==1||order.checkout_confirmation.root_register_id!==e.fiscalRegisterID||!Number.isSafeInteger(order.version)||order.version<e.sourceVersion||!Number.isSafeInteger(order.fiscal_version)||(order.fiscal_version??0)<operation.version||!Array.isArray(order.allowed_actions)||order.allowed_actions.some(a=>typeof a!=='string')||canonical(order.total)!==canonical(e.total)||order.checkout_confirmation.payments_sha256!==preparedPaymentsDigest(e.payments))throw Error('SOURCE_CHECKOUT_PROOF_MISMATCH');
 if(observed?.state==='FISCALIZED'&&(order.fiscal_version??0)<observed.version)throw Error('SOURCE_CHECKOUT_PROOF_VERSION_MISMATCH');
 let effectiveOperation=operation;
 if(observed?.state==='FISCALIZED'&&(operation.state!=='FISCALIZED'||operation.version<observed.version)&&['COMPLETED','REVERSED'].includes(order.state)&&order.receipt_reference===observed.fiscal_reference)effectiveOperation=observed;
 let kind:'FISCALIZED'|'FAILED'|'PENDING'='PENDING';
 if(effectiveOperation.state==='FISCALIZED'){
  if(!['COMPLETED','REVERSED'].includes(order.state)||!required(effectiveOperation.fiscal_reference)||order.receipt_reference!==effectiveOperation.fiscal_reference)throw Error('SOURCE_CHECKOUT_RECEIPT_MISMATCH');
  kind='FISCALIZED';
 }else if(operation.state==='FAILED'||operation.state==='CANCELLED'){
  if(order.state!=='FAILED'||operation.fiscal_reference||order.receipt_reference)throw Error('SOURCE_CHECKOUT_REJECTION_MISMATCH');
  kind='FAILED';
 }
 if(kind!=='PENDING'){alive(live);await journal.remove(e.sourceOrderID);alive(live);}
 return {kind,operation:effectiveOperation,order};
}
