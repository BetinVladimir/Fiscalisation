import {payloadDigest} from './nativeExecutionAuthority.ts';
import {preparedPaymentsDigest} from './cloudCheckoutConfirmation.ts';
import {recoverCheckoutReadOnly,type RecoveryScope} from './checkoutRecovery.ts';
import {collectCursorPages} from './pagination.ts';
import type {DurableCheckout} from './checkoutJournal.ts';
import type {SourceOrder} from './sourceOrders.ts';
export const legacyRecoveryReasons=['SOURCE_LINKAGE_REQUIRED','INSUFFICIENT_SIGNED_SOURCE_ANCHORS','EXPLICIT_DISCOVERY_REQUIRED','AMBIGUOUS_SOURCE_LINKAGE','SOURCE_LINKAGE_NOT_FOUND','SOURCE_CAPABILITY_DENIED','OWNER_SCOPED_SOURCE_PROOF'] as const;
export type LegacyRecoveryReason=typeof legacyRecoveryReasons[number];
export type LegacyRecoveryRecord={version:1;journal_sha256:string;status:'QUARANTINED'|'LINKED';reason:LegacyRecoveryReason;source_order_id?:string;source_context?:{shift_id:string;register_id:string;operator_code:string;external_id:string}};
const nonempty=(v:unknown):v is string=>typeof v==='string'&&v.trim().length>0;
export function validateLegacyRecoveryRecord(value:unknown):LegacyRecoveryRecord{
 const v=value as LegacyRecoveryRecord;
 if(!v||typeof v!=='object'||Array.isArray(v)||Object.keys(v).some(k=>!['version','journal_sha256','status','reason','source_order_id','source_context'].includes(k))||v.version!==1||typeof v.journal_sha256!=='string'||!/^[a-f0-9]{64}$/.test(v.journal_sha256)||!['QUARANTINED','LINKED'].includes(v.status)||!legacyRecoveryReasons.includes(v.reason))throw Error('LEGACY_RECOVERY_RECORD_CORRUPT');
 if(v.status==='QUARANTINED'&&(v.reason==='OWNER_SCOPED_SOURCE_PROOF'||v.source_order_id!==undefined||v.source_context!==undefined))throw Error('LEGACY_RECOVERY_RECORD_CORRUPT');
 if(v.status==='LINKED'){
  const c=v.source_context;
  if(v.reason!=='OWNER_SCOPED_SOURCE_PROOF'||!nonempty(v.source_order_id)||!c||typeof c!=='object'||Array.isArray(c)||Object.keys(c).sort().join(',')!=='external_id,operator_code,register_id,shift_id'||![c.shift_id,c.register_id,c.operator_code,c.external_id].every(nonempty))throw Error('LEGACY_RECOVERY_RECORD_CORRUPT');
 }
 return v;
}
export const legacyJournalDigest=(durable:DurableCheckout)=>payloadDigest(durable);
/** Global storage does not prove ownership. Without exact current signed
 * operator/register anchors, export only hash and bounded quarantine diagnostics. */
export function legacyRecoveryExport(d:DurableCheckout,record?:LegacyRecoveryRecord,scope?:RecoveryScope){
 const valid=record?validateLegacyRecoveryRecord(record):undefined;
 const digest=legacyJournalDigest(d);
 if(valid&&valid.journal_sha256!==digest)throw Error('LEGACY_RECOVERY_RECORD_MISMATCH');
 const diagnostic={kind:'MINIPOS_LEGACY_RECOVERY_QUARANTINE',version:1,journal_sha256:digest,status:valid?.status??'QUARANTINED',reason:valid?.reason??'SOURCE_LINKAGE_REQUIRED'};
 const signed=d.signed_intent;
 const owned=!!scope&&nonempty(scope.operatorCode)&&nonempty(scope.fiscalRegisterID)&&signed?.operator_code===scope.operatorCode&&signed?.register_id===scope.fiscalRegisterID&&signed?.intent_id===d.plan.client_operation_id&&signed?.receipt_session_id===d.plan.receipt_session_id;
 if(!owned)return diagnostic;
 if(typeof d.plan.expected_total?.amount!=='string'||!/^\d+\.\d{2}$/.test(d.plan.expected_total.amount)||d.plan.expected_total.currency!=='EUR')throw Error('LEGACY_EXPORT_MONETARY_INVALID');
 return{...diagnostic,journal_id:d.sale_id,created_at:d.created_at,phase:d.phase??'LEGACY',operation_id:d.plan.client_operation_id,receipt_session_id:d.plan.receipt_session_id,total:{amount:d.plan.expected_total.amount,currency:d.plan.expected_total.currency},payments_sha256:preparedPaymentsDigest(d.plan.payments),...(valid?.source_order_id?{source_order_id:valid.source_order_id}:{})};
}
type Store={loadLegacyRecovery(id:string):Promise<LegacyRecoveryRecord|null>;saveLegacyRecovery(id:string,record:LegacyRecoveryRecord):Promise<void>;remove(id:string):Promise<void>};
const text=(v:unknown):v is string=>typeof v==='string'&&v.trim().length>0;
export async function recoverLegacyCheckout(options:{durable:DurableCheckout;scope:RecoveryScope;read:<T>(path:string)=>Promise<T>;journal:Store;live:()=>boolean;discover?:boolean}){
 const {durable:d,scope,read,journal,live}=options;
 const alive=()=>{if(!live())throw Error('CHECKOUT_AUTHORITY_CHANGED')};
 if(d.source_order_id)throw Error('LEGACY_RECOVERY_ONLY');
 alive();const digest=legacyJournalDigest(d);let record=await journal.loadLegacyRecovery(d.sale_id);alive();
 if(record)record=validateLegacyRecoveryRecord(record);
 if(record&&record.journal_sha256!==digest)throw Error('LEGACY_RECOVERY_RECORD_MISMATCH');
 const signed=d.signed_intent;
 const anchored=!!signed&&[signed.register_id,signed.operator_code,signed.client_sale_surrogate_id,signed.server_sale_id].every(text)&&signed.register_id===scope.fiscalRegisterID&&signed.operator_code===scope.operatorCode&&signed.intent_id===d.plan.client_operation_id&&signed.receipt_session_id===d.plan.receipt_session_id;
 const quarantine=async(reason:LegacyRecoveryReason)=>{alive();record={version:1,journal_sha256:digest,status:'QUARANTINED',reason};await journal.saveLegacyRecovery(d.sale_id,record);alive();return{kind:'QUARANTINED' as const,record,report:legacyRecoveryExport(d,record,scope)}};
 if(!anchored)return quarantine('INSUFFICIENT_SIGNED_SOURCE_ANCHORS');
 if(!record?.source_order_id){
  if(!options.discover)return quarantine('EXPLICIT_DISCOVERY_REQUIRED');
  const orders=await collectCursorPages<SourceOrder>('/orders',async path=>{alive();const result=await read<any>(path);alive();return result});
  const candidates=orders.filter(o=>text(o.id)&&o.operator_code===scope.operatorCode&&o.external_id===signed!.client_sale_surrogate_id&&o.fiscal_sale_id===signed!.server_sale_id&&o.checkout_operation_id===d.plan.client_operation_id&&o.receipt_session_id===d.plan.receipt_session_id&&o.checkout_confirmation?.root_register_id===scope.fiscalRegisterID&&o.checkout_confirmation.payments_sha256===preparedPaymentsDigest(d.plan.payments));
  if(candidates.length!==1)return quarantine(candidates.length?'AMBIGUOUS_SOURCE_LINKAGE':'SOURCE_LINKAGE_NOT_FOUND');
  const source=candidates[0];record={version:1,journal_sha256:digest,status:'LINKED',reason:'OWNER_SCOPED_SOURCE_PROOF',source_order_id:source.id,source_context:{shift_id:source.shift_id,register_id:source.register_id,operator_code:source.operator_code,external_id:source.external_id}};
 }
 const sourceID=record.source_order_id!;
 // Only this transient projection carries a source ID. Frozen persisted bytes
 // and its monetary plan stay untouched; cleanup always targets original key.
 const projected={...d,sale_id:sourceID,source_order_id:sourceID,source_context:record.source_context};
 let terminal=false;
 const result=await recoverCheckoutReadOnly({durable:projected,scope,read,live,journal:{async remove(){terminal=true}}});alive();
 // A LINKED record is persisted only after exact capability and root/source
 // confirmation checks, never from list metadata alone or a denied capability.
 if(result.kind==='DENIED')return quarantine('SOURCE_CAPABILITY_DENIED');
 await journal.saveLegacyRecovery(d.sale_id,record);alive();
 if(terminal){await journal.remove(d.sale_id);alive();}
 return{...result,record};
}
