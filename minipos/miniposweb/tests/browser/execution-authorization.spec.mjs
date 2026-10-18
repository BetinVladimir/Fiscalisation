import { test, expect } from '@playwright/test';
import { createPublicKey, verify } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
const scope={backend:'https://minipos.example',tenant:'tenant',actorSubject:'issuer-subject-namespaced',appInstance:'app',sessionID:'authenticated-session'};
test.beforeEach(async({page})=>{await page.goto('/');});
test('actual CryptoKey persists, is nonextractable and converges across tabs',async({page,context})=>{
 const first=await page.evaluate(async scope=>{const m=await import('/src/fiscal/executionAuthorization');const key=await m.browserSigningKey(scope);let failed=false;try{await crypto.subtle.exportKey('pkcs8',key.privateKey);}catch{failed=true;}return {pub:key.publicKey,extractable:key.privateKey.extractable,failed};},scope);
 await page.reload();const other=await context.newPage();await other.goto('/');
 const keys=await Promise.all([page,other].map(p=>p.evaluate(async scope=>(await (await import('/src/fiscal/executionAuthorization')).browserSigningKey(scope)).publicKey,scope)));
 expect(keys).toEqual([first.pub,first.pub]);expect(first.extractable).toBe(false);expect(first.failed).toBe(true);
 const foreign=await page.evaluate(async scope=>(await (await import('/src/fiscal/executionAuthorization')).browserSigningKey({...scope,sessionID:'other-session'})).publicKey,scope);expect(foreign).not.toBe(first.pub);
});
async function contextInPage(page,scope){await page.evaluate(async scope=>{
 const m=await import('/src/fiscal/executionAuthorization');const {canonical}=await import('/src/fiscal/goCanonical');const pair=await m.browserSigningKey(scope);const now=Math.floor(Date.now()/1000);
 const claims={kind:'EXECUTION_AUTHORIZATION',version:1,authorization_id:'auth',workstation_session_id:'worksession',tenant_id:scope.tenant,location_id:'location',register_id:'register',edge_device_id:'edge',fiscal_device_id:'fiscal',binding_sha256:'a'.repeat(64),binding_generation:1,operator_id:'fiscal-operator-uuid',operator_code:'A001',actor_subject:scope.actorSubject,app_instance_id:scope.appInstance,client_signing_public_key_der_base64:pair.publicKey,scope:'fiscal.execute',allowed_commands:['SALE_FINALIZE'],nbf:now-1,exp:now+600,authority:{binding_sha256:'a'.repeat(64),range_start:1,range_end:1000,unp_prefix:'AB123456'}};
 const json=canonical(claims),envelope={algorithm:'ES256_DER',key_id:'fixture-root',payload:m.base64url(new TextEncoder().encode(json)),sha256:await m.sha256(json),signature:'fixture-root-envelope-browser-does-not-verify'};
 window.fixtureContext=await m.cacheExecutionAuthorization(scope,envelope);
 },scope);}
test('actual browser P256 PoP DER verifies independently with OpenSSL and binds plan',async({page})=>{
 await contextInPage(page,scope);
 const result=await page.evaluate(async()=>{const m=await import('/src/fiscal/executionAuthorization');const {canonical}=await import('/src/fiscal/goCanonical');const c=window.fixtureContext;const intent={intent_id:'operation',receipt_session_id:'receipt',payload_sha256:'b'.repeat(64),server_sale_id:null,client_sale_surrogate_id:'surrogate',command:'SALE_FINALIZE'};const proof=await m.signIntentProof(c,intent);const transcript='BEELOY-EXECUTION-INTENT-PROOF-V1\n'+canonical({kind:'EXECUTION_INTENT_PROOF',version:1,authorization_sha256:c.envelope.sha256,operation_id:intent.intent_id,receipt_session_id:intent.receipt_session_id,payload_sha256:intent.payload_sha256,server_sale_id:null,client_sale_surrogate_id:'surrogate',binding_sha256:c.claims.binding_sha256,binding_generation:1,command_type:'SALE_FINALIZE'});return {proof,transcript,key:c.claims.client_signing_public_key_der_base64};});
 const key=createPublicKey({key:Buffer.from(result.key,'base64url'),format:'der',type:'spki'});const sig=Buffer.from(result.proof.signature,'base64url');
 expect(verify('sha256',Buffer.from(result.transcript),key,sig)).toBe(true);expect(verify('sha256',Buffer.from(result.transcript.replace('surrogate','changed')),key,sig)).toBe(false);
});
test('logout fences stale context and issuance cache and does not revive key',async({page})=>{
 await contextInPage(page,scope);
 const result=await page.evaluate(async scope=>{const m=await import('/src/fiscal/executionAuthorization');const context=window.fixtureContext;await m.clearExecutionAuthorization(scope);const errors=[];for(const task of [()=>m.browserSigningKey(scope),()=>m.cacheExecutionAuthorization(scope,context.envelope),()=>m.signIntentProof(context,{intent_id:'operation',receipt_session_id:'receipt',payload_sha256:'b'.repeat(64),server_sale_id:null,client_sale_surrogate_id:'s',command:'SALE_FINALIZE'})])try{await task();errors.push('unexpected');}catch(e){errors.push(e.message);}return {errors,cached:!!await m.cachedExecutionAuthorization(scope)};},scope);
 expect(result.errors).toEqual(['AUTHENTICATED_SESSION_REVOKED','AUTHENTICATED_SESSION_REVOKED','AUTHENTICATED_SESSION_REVOKED']);expect(result.cached).toBe(false);
});
test('logout fences a key generation already in flight',async({page})=>{
 const result=await page.evaluate(async scope=>{const m=await import('/src/fiscal/executionAuthorization');const original=crypto.subtle.generateKey.bind(crypto.subtle);let release;const gate=new Promise(r=>release=r);let entered;const started=new Promise(r=>entered=r);crypto.subtle.generateKey=async(...args)=>{entered();await gate;return original(...args);};const pending=m.browserSigningKey(scope).then(()=> 'unexpected',e=>e.message);await started;await m.clearExecutionAuthorization(scope);release();const error=await pending;crypto.subtle.generateKey=original;return error;},scope);expect(result).toBe('AUTHENTICATED_SESSION_REVOKED');
});

for(const online of [false,true])test(`actual Go root issuer and web builder preserve ${online?'server sale/UNP':'offline null sale/surrogate'}`,async({page})=>{
 const scope={backend:'http://minipos.test',tenant:'tenant-1',actorSubject:'subject-1',appInstance:'app-1',sessionID:'browser-source-session'};
 const publicKey=await page.evaluate(async scope=>(await (await import('/src/fiscal/executionAuthorization')).browserSigningKey(scope)).publicKey,scope);
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),'browser-authority-'));const fixturePath=path.join(directory,'browser-issuer-fixture.json');
 execFileSync('go',['test','./internal/domain','-run','^TestBrowserExecutionAuthorizationFixture$','-count=1'],{cwd:path.resolve('../../fiscal-backend'),env:{...process.env,GOCACHE:process.env.GOCACHE||'/private/tmp/fiscal-web-authority-go-cache',BROWSER_CLIENT_SPKI:publicKey,BROWSER_AUTHORITY_FIXTURE:fixturePath,BROWSER_AUTHORITY_ONLINE:online?'1':'0'},stdio:'pipe'});
 const fixture=JSON.parse(fs.readFileSync(fixturePath,'utf8'));
 fixture.command=await page.evaluate(async({scope,fixture})=>{const m=await import('/src/fiscal/executionAuthorization');const {buildIntent}=await import('/src/fiscal/intentBuilder');const context=await m.cacheExecutionAuthorization(scope,fixture.authorization);const c=fixture.command,p=c.canonical_payload,a=context.claims;return buildIntent({execution:context,tenantId:a.tenant_id,locationId:a.location_id,registerId:a.register_id,edgeDeviceId:a.edge_device_id,bindingGeneration:a.binding_generation,operatorCode:a.operator_code,command:'SALE_FINALIZE',operationId:c.intent_id,receiptSessionId:c.receipt_session_id,surrogateId:c.client_sale_surrogate_id,saleId:a.unp_authority?.server_sale_id,serverPreparedPayload:a.unp_authority?p:undefined,items:p.items.map(({line_id,...item})=>({item_id:line_id,...item})),payments:p.payments});},{scope,fixture});
 expect(fixture.command.server_sale_id).toBe(online?'online-sale':null);expect(fixture.command.canonical_payload.server_sale_id).toBe(online?'online-sale':null);expect(fixture.command.client_sale_surrogate_id).toBeTruthy();if(online)expect(fixture.command.canonical_payload.unp).toBe('AB123456-A001-0005000');expect(fixture.command.canonical_payload.operator_id).toBe('operator-1');expect(fixture.command.canonical_payload.operator_code).toBe('A001');
 expect(fixture.command.client_proof.algorithm).toBe('ES256_DER');
 fs.writeFileSync(fixturePath,JSON.stringify(fixture));
 if(process.env.BROWSER_AUTHORITY_FIXTURE_OUTPUT)fs.copyFileSync(fixturePath,process.env.BROWSER_AUTHORITY_FIXTURE_OUTPUT+(online?'.online':''));
 console.log('Browser-owned root-issued authority fixture: '+fixturePath);
});

for(const failure of ['identity401','issuance403','network','503','409'])test(`manager lifecycle ${failure} preserves only bounded authority or revokes confirmed denial`,async({page})=>{
 await contextInPage(page,scope);
 const result=await page.evaluate(async({scope,failure})=>{
  const {ExecutionAuthorizationManager}=await import('/src/fiscal/executionManager');const {ApiError}=await import('/src/api/client');const m=await import('/src/fiscal/executionAuthorization');
  const envelope=window.fixtureContext.envelope;
  const api={baseUrl:scope.backend,appInstance:scope.appInstance,executionIdentity:async()=>({tenant_id:scope.tenant,actor_subject:scope.actorSubject,source_session_id:scope.sessionID,app_instance_id:scope.appInstance}),executionAuthorization:async()=>({authorization:envelope,transport_token:'bounded-token'})};
  const manager=new ExecutionAuthorizationManager(api,{tenant:'source-tenant',employee:'source-employee',sessionID:scope.sessionID,shiftID:'source-shift'});
  const old=(await manager.issue()).context;
  if(failure==='identity401')api.executionIdentity=async()=>{throw new ApiError(401,'source revoked')};
  else api.executionAuthorization=async()=>{if(failure==='network')throw new TypeError('Failed to fetch');throw new ApiError(failure==='issuance403'?403:Number(failure),'issuer refused')};
  let error='';try{await manager.issue()}catch(e){error=e.message}
  const cached=await m.cachedExecutionAuthorization(scope);let signed=false;try{await m.signIntentProof(old,{intent_id:'operation',receipt_session_id:'receipt',payload_sha256:'b'.repeat(64),server_sale_id:null,client_sale_surrogate_id:'s',command:'SALE_FINALIZE'});signed=true}catch{}
  const original=Date.now;Date.now=()=>old.claims.exp*1000;const expired=await m.cachedExecutionAuthorization(scope);Date.now=original;
  let revived=false;if(failure==='identity401'||failure==='issuance403')try{await manager.issue();revived=true}catch{}
  return {error,cached:!!cached,signed,expired:!!expired,revived};
 },{scope,failure});
 expect(result.error).not.toBe('');expect(result.expired).toBe(false);
 if(failure==='identity401'||failure==='issuance403'){expect(result.cached).toBe(false);expect(result.signed).toBe(false);expect(result.revived).toBe(false)}
 else {expect(result.cached).toBe(true);expect(result.signed).toBe(true)}
});
