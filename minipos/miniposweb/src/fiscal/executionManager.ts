import {browserSigningKey,cacheExecutionAuthorization,cachedExecutionAuthorization,clearExecutionAuthorization,type AuthorizedContext,type ExecutionScope} from './executionAuthorization';
import {ApiError,type MiniPosClient} from '../api/client';
import type {SalePayment} from '../types';
import {canonical} from './goCanonical';
export class ExecutionAuthorizationManager {
 private scope?:ExecutionScope;
 private readonly index:string;
 constructor(private api:MiniPosClient,private source:{tenant:string;employee:string;sessionID:string;shiftID:string}){this.index='minipos-execution-scope:'+canonical([api.baseUrl,source.tenant,source.employee,source.sessionID,api.appInstance,source.shiftID]);}
 private assertLive(){if(localStorage.getItem(this.index+':revoked'))throw new Error('AUTHENTICATED_SESSION_REVOKED');}
 async restore():Promise<AuthorizedContext|undefined>{this.assertLive();let scope:ExecutionScope;try{scope=JSON.parse(localStorage.getItem(this.index)||'null');}catch{return;}
 if(!scope||scope.sessionID!==this.source.sessionID||scope.backend!==this.api.baseUrl||scope.appInstance!==this.api.appInstance)return;
 this.scope=scope;const context=await cachedExecutionAuthorization(scope);this.assertLive();return context;
 }
 async issue(order?:{id:string;payments:SalePayment[]}):Promise<{context:AuthorizedContext;canonicalPayload?:Record<string,unknown>}>{
  try{return await this.issueRequest(order);}catch(error){if(error instanceof ApiError&&(error.status===401||error.status===403))await this.revoke();throw error;}
 }
 private async issueRequest(order?:{id:string;payments:SalePayment[]}):Promise<{context:AuthorizedContext;canonicalPayload?:Record<string,unknown>}>{
 this.assertLive();const identity=await this.api.executionIdentity(this.source.shiftID);this.assertLive();
 if(identity.source_session_id!==this.source.sessionID||identity.app_instance_id!==this.api.appInstance||!identity.tenant_id||!identity.actor_subject)throw new Error('EXECUTION_IDENTITY_MISMATCH');
 const scope={backend:this.api.baseUrl,tenant:identity.tenant_id,actorSubject:identity.actor_subject,appInstance:this.api.appInstance,sessionID:this.source.sessionID};this.scope=scope;
 localStorage.setItem(this.index,JSON.stringify(scope));
 try{const key=await browserSigningKey(scope);this.assertLive();const response=await this.api.executionAuthorization({shift_id:this.source.shiftID,client_signing_public_key_der_base64:key.publicKey,...(order?{order_id:order.id,payments:order.payments}:{})},crypto.randomUUID());this.assertLive();const context=await cacheExecutionAuthorization(scope,response.authorization,response.transport_token);this.assertLive();
 if(order&&!context.claims.unp_authority)throw new Error('SERVER_PREPARED_AUTHORITY_REQUIRED');
 return {context,canonicalPayload:response.canonical_payload};
 }catch(error){if(localStorage.getItem(this.index+':revoked'))await clearExecutionAuthorization(scope);throw error;}
 }
 async revoke(){localStorage.setItem(this.index+':revoked','1');let scope=this.scope;try{scope??=JSON.parse(localStorage.getItem(this.index)||'null');}catch{}localStorage.removeItem(this.index);if(scope)await clearExecutionAuthorization(scope);}
}
