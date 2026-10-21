import test from 'node:test';
import assert from 'node:assert/strict';
import {NativeExecutionManager} from './nativeExecutionManager.ts';
import {NativeExecutionAuthority,payloadDigest} from './nativeExecutionAuthority.ts';
import {base64urlEncode,hash256} from './portableCrypto.ts';
import {canonical} from './goCanonical.ts';
const source={backend:'https://source.test',token:'fixture-token',appInstance:'app',shiftID:'shift'};
const identity={tenant_id:'tenant',actor_subject:'actor',source_session_id:base64urlEncode(hash256(new TextEncoder().encode(source.token))),app_instance_id:'app'};
function setup(){
 const values=new Map<string,string>();let revoked=0;const posts:RequestInit[]=[];
 const store={async getItem(k:string){return values.get(k)??null;},async setItem(k:string,v:string){values.set(k,v);},async removeItem(k:string){values.delete(k);}};
 const authority=new NativeExecutionAuthority({async prepare(){return 'public-fixture';},async sign(){throw Error('not signing in issuance test');},async revoke(){revoked++;}},store,()=>1000);
 const payload={server_sale_id:'root-sale',external_id:'surrogate'};
 const claims={kind:'EXECUTION_AUTHORIZATION',version:1,authorization_id:'grant',workstation_session_id:'session',tenant_id:'tenant',location_id:'location',register_id:'register',edge_device_id:'edge',fiscal_device_id:'fiscal',binding_sha256:'a'.repeat(64),binding_generation:1,operator_id:'operator',operator_code:'A001',actor_subject:'actor',app_instance_id:'app',client_signing_public_key_der_base64:'public-fixture',scope:'fiscal.execute',allowed_commands:['SALE_FINALIZE'],nbf:990,exp:1100,authority:{},unp_authority:{source:'SERVER_ASSIGNED',server_sale_id:'root-sale',unp:'AB123456-A001-0000001',payload_sha256:payloadDigest(payload),operation_id:'root-op',receipt_session_id:'root-receipt'}};
 const response={authorization:{algorithm:'ES256_DER',key_id:'issuer',signature:'fixture-not-cryptographic-evidence',payload:base64urlEncode(new TextEncoder().encode(canonical(claims))),sha256:payloadDigest(claims)},canonical_payload:payload};
 let get=async()=>identity;let post=async()=>response;
 const request=async<T>(_path:string,init?:RequestInit):Promise<T>=>{if(init?.method==='POST'){posts.push(init);return await post() as T;}return await get() as T;};
 const manager=()=>new NativeExecutionManager(authority,store,request,source,()=>{throw Error('must use durable request key');});
 return {manager,posts,values,response,setGet(fn:typeof get){get=fn;},setPost(fn:typeof post){post=fn;},get revoked(){return revoked;}};
}
test('lost preparation response retries the same durable key and exact source order',async()=>{
 const x=setup();let first=true;x.setPost(async()=>{if(first){first=false;throw Error('response lost');}return x.response;});
 await assert.rejects(()=>x.manager().issue({id:'source-order',payments:[]},'durable-key'),/response lost/);
 const result=await x.manager().issue({id:'source-order',payments:[]},'durable-key');
 assert.equal(result.context.claims.unp_authority?.operation_id,'root-op');assert.equal(result.canonicalPayload?.server_sale_id,'root-sale');
 assert.equal(x.posts.length,2);assert.equal(x.posts[0].body,x.posts[1].body);
 for(const p of x.posts){assert.equal((p.headers as Record<string,string>)['Idempotency-Key'],'durable-key');assert.equal(JSON.parse(String(p.body)).order_id,'source-order');}
});
test('foreign identity is refused before preparation POST',async()=>{
 const x=setup();x.setGet(async()=>({...identity,source_session_id:'foreign'}));
 await assert.rejects(()=>x.manager().issue({id:'source',payments:[]},'key'),/IDENTITY_MISMATCH/);assert.equal(x.posts.length,0);
});
test('403 revokes durably across manager reconstruction',async()=>{
 const x=setup();x.setPost(async()=>{throw Object.assign(Error('forbidden'),{status:403});});
 await assert.rejects(()=>x.manager().issue({id:'source',payments:[]},'key'),/forbidden/);assert.equal(x.revoked,1);
 await assert.rejects(()=>x.manager().issue({id:'source',payments:[]},'key'),/REVOKED/);assert.equal(x.posts.length,1);
});
test('revocation fences a successful preparation response already in flight',async()=>{
 const x=setup();let release!:(v:typeof x.response)=>void;let entered!:()=>void;const enteredPost=new Promise<void>(r=>{entered=r;});
 x.setPost(()=>{entered();return new Promise(r=>{release=r;});});const manager=x.manager();
 const pending=manager.issue({id:'source',payments:[]},'key');const rejected=assert.rejects(pending,/REVOKED/);
 await enteredPost;await manager.revoke();release(x.response);await rejected;
 await assert.rejects(()=>x.manager().restore(),/REVOKED/);assert.equal(x.revoked,1);
});
