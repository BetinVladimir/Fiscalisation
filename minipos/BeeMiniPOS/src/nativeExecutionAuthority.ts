import { canonical } from './goCanonical.ts';
import { base64urlDecode, base64urlEncode, hash256 } from './portableCrypto.ts';
import type { CheckoutPlan } from './checkoutPlan.ts';
export type ExecutionScope={backend:string;tenant:string;actorSubject:string;appInstance:string;sessionID:string};
export type AuthorizationEnvelope={algorithm:'ES256_DER';key_id:string;payload:string;sha256:string;signature:string};
export type ExecutionClaims={kind:'EXECUTION_AUTHORIZATION';version:1;authorization_id:string;workstation_session_id:string;tenant_id:string;location_id:string;register_id:string;edge_device_id:string;fiscal_device_id:string;binding_sha256:string;binding_generation:number;operator_id:string;operator_code:string;actor_subject:string;app_instance_id:string;client_signing_public_key_der_base64:string;scope:'fiscal.execute';allowed_commands:string[];nbf:number;exp:number;authority:{binding_sha256:string;range_start:number;range_end:number;unp_prefix:string};unp_authority?:{source:'SERVER_ASSIGNED';server_sale_id:string;unp:string;payload_sha256:string;operation_id:string;receipt_session_id:string}};
export type AuthorizedContext={scope:ExecutionScope;envelope:AuthorizationEnvelope;claims:ExecutionClaims;publicKey:string;transportToken?:string};
export interface NativeKeyProvider {prepare(scope:string):Promise<string>;sign(scope:string,publicKey:string,message:string):Promise<string>;revoke(scope:string):Promise<void>}
export interface AuthorityStore {getItem(key:string):Promise<string|null>;setItem(key:string,value:string):Promise<void>;removeItem(key:string):Promise<void>}
export const payloadDigest=(value:unknown)=>[...hash256(new TextEncoder().encode(canonical(value)))].map(v=>v.toString(16).padStart(2,'0')).join('');
export const executionScopeID=(s:ExecutionScope)=>canonical([s.backend,s.tenant,s.actorSubject,s.appInstance,s.sessionID]);
const cache=(s:ExecutionScope)=>'beeminipos:execution:v1:'+executionScopeID(s);
/** JS sees SPKI and DER only. Native code serializes signing/revocation and persists its own tombstone. */
export class NativeExecutionAuthority {
 private native:NativeKeyProvider; private store:AuthorityStore; private now:()=>number;
 constructor(native:NativeKeyProvider,store:AuthorityStore,now=()=>Math.floor(Date.now()/1000)){this.native=native;this.store=store;this.now=now;}
 private async active(scope:ExecutionScope){if(!scope.backend||!scope.tenant||!scope.actorSubject||!scope.appInstance||!scope.sessionID)throw Error('AUTHENTICATED_SESSION_REQUIRED');if(await this.store.getItem(cache(scope)+':revoked'))throw Error('AUTHENTICATED_SESSION_REVOKED');}
 async publicKey(scope:ExecutionScope){await this.active(scope);const key=await this.native.prepare(executionScopeID(scope));await this.active(scope);return key;}
 async accept(scope:ExecutionScope,envelope:AuthorizationEnvelope,transportToken?:string):Promise<AuthorizedContext>{
  const publicKey=await this.publicKey(scope);
  if(envelope.algorithm!=='ES256_DER'||!envelope.key_id||!envelope.signature||!/^[0-9a-f]{64}$/.test(envelope.sha256))throw Error('EXECUTION_AUTHORIZATION_INVALID');
  const json=new TextDecoder('utf-8',{fatal:true}).decode(base64urlDecode(envelope.payload)),claims=JSON.parse(json) as ExecutionClaims;
  const now=this.now();
  if(canonical(claims)!==json||payloadDigest(claims)!==envelope.sha256||claims.kind!=='EXECUTION_AUTHORIZATION'||claims.version!==1||claims.scope!=='fiscal.execute'||claims.tenant_id!==scope.tenant||claims.actor_subject!==scope.actorSubject||claims.app_instance_id!==scope.appInstance||claims.client_signing_public_key_der_base64!==publicKey||!claims.authorization_id||!claims.workstation_session_id||!claims.operator_id||!/^[A-Za-z0-9]{4}$/.test(claims.operator_code)||!Number.isSafeInteger(claims.binding_generation)||claims.binding_generation<1||!Number.isSafeInteger(claims.nbf)||!Number.isSafeInteger(claims.exp)||claims.exp<=claims.nbf||claims.exp-claims.nbf>900||now<claims.nbf||now>=claims.exp||!Array.isArray(claims.allowed_commands)||!claims.allowed_commands.includes('SALE_FINALIZE'))throw Error('EXECUTION_AUTHORIZATION_SCOPE_OR_EXPIRY');
  const context={scope,envelope,claims,publicKey,transportToken};await this.active(scope);
  // An online sale grant must never evict a bounded offline lease.
  if(!claims.unp_authority){await this.store.setItem(cache(scope),JSON.stringify({envelope,transportToken}));await this.active(scope);}
  return context;
 }
 async restore(scope:ExecutionScope):Promise<AuthorizedContext|undefined>{try{await this.active(scope);const raw=await this.store.getItem(cache(scope));if(!raw)return;const saved=JSON.parse(raw);return await this.accept(scope,saved.envelope,saved.transportToken);}catch{return;}}
 async revoke(scope:ExecutionScope){await this.store.setItem(cache(scope)+':revoked','1');await this.native.revoke(executionScopeID(scope));await this.store.removeItem(cache(scope));}
 async signIntent(context:AuthorizedContext,intent:{intent_id:string;receipt_session_id:string;payload_sha256:string;server_sale_id:string|null;client_sale_surrogate_id:string|null;command:string}){
  await this.active(context.scope);const c=context.claims,now=this.now();if(now<c.nbf||now>=c.exp||!c.allowed_commands.includes(intent.command))throw Error('EXECUTION_AUTHORIZATION_EXPIRED_OR_SCOPE');
  const proof={kind:'EXECUTION_INTENT_PROOF',version:1,authorization_sha256:context.envelope.sha256,operation_id:intent.intent_id,receipt_session_id:intent.receipt_session_id,payload_sha256:intent.payload_sha256,server_sale_id:intent.server_sale_id,client_sale_surrogate_id:intent.client_sale_surrogate_id,binding_sha256:c.binding_sha256,binding_generation:c.binding_generation,command_type:intent.command};
  const message=base64urlEncode(new TextEncoder().encode('BEELOY-EXECUTION-INTENT-PROOF-V1\n'+canonical(proof)));
  const signature=await this.native.sign(executionScopeID(context.scope),context.publicKey,message);await this.active(context.scope);
  return {algorithm:'ES256_DER' as const,signature};
 }
}
export async function buildAuthorizedFinalizeIntent(service:NativeExecutionAuthority,context:AuthorizedContext,plan:CheckoutPlan,route:{tenant_id:string;location_id:string;register_id:string;edge_id:string;binding_version:number},sale:{external_id:string;lines:unknown[]},preparedPayload?:Record<string,unknown>){
 const c=context.claims;
 if(c.tenant_id!==route.tenant_id||c.location_id!==route.location_id||c.register_id!==route.register_id||c.edge_device_id!==route.edge_id||c.binding_generation!==route.binding_version)throw Error('EXECUTION_AUTHORIZATION_ROUTE_MISMATCH');
 const assigned=c.unp_authority;
 if(assigned&&(!preparedPayload||assigned.source!=='SERVER_ASSIGNED'))throw Error('SERVER_PREPARED_PAYLOAD_REQUIRED');
 const canonical_payload=assigned?preparedPayload!:{currency:'EUR',server_sale_id:null,external_id:sale.external_id,location_id:route.location_id,operator_id:c.operator_id,operator_code:c.operator_code,items:sale.lines,payments:plan.payments,metadata:{}};
 const payload_sha256=payloadDigest(canonical_payload);
 if(assigned&&(payload_sha256!==assigned.payload_sha256||canonical_payload.server_sale_id!==assigned.server_sale_id||canonical_payload.external_id!==sale.external_id||canonical_payload.operator_id!==c.operator_id||canonical_payload.operator_code!==c.operator_code||canonical_payload.location_id!==c.location_id))throw Error('SERVER_PREPARED_PAYLOAD_MISMATCH');
 const intent={protocol_version:'1.0',intent_id:assigned?.operation_id??plan.client_operation_id,action:'SALE_FINALIZE',command:'SALE_FINALIZE',client_operation_id:assigned?.operation_id??plan.client_operation_id,receipt_session_id:assigned?.receipt_session_id??plan.receipt_session_id,server_sale_id:assigned?.server_sale_id??null,client_sale_surrogate_id:sale.external_id,tenant_id:c.tenant_id,location_id:c.location_id,register_id:c.register_id,edge_device_id:c.edge_device_id,binding_generation:c.binding_generation,operator_code:c.operator_code,canonical_payload,payload_sha256,execution_authorization:context.envelope};
 return {...intent,client_proof:await service.signIntent(context,intent)};
}
