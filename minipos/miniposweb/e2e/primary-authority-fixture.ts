import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdtempSync,readFileSync,writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import path from 'node:path';
import type {Page} from '@playwright/test';
import {resultFixture} from './fiscal-operation-result-fixture';
import {PRODUCTS,SESSION,SHIFT,ORDER,TEST_JWT} from './helpers';
export async function primaryAuthorityFixture(page:Page,options:{dispatchState?:'UNKNOWN'}={}){
 const oracle=process.env.MINIPOS_AUTHORITY_ORACLE;if(!oracle)throw Error('Real Go authority oracle binary required');
 const base=await resultFixture(page,'sale',{products:PRODUCTS.map((p,i)=>({...p,name:i===0?'Test coffee':'Test water'}))});
 const calls:any[]=[],issuances:any[]=[];let online:any,dispatches=0,resolved=false;
 const sessionID='authenticated-source-session-0000000000000001';
 await page.route('**/public/v1/minipos/**',async route=>{
  const req=route.request(),url=new URL(req.url()),suffix=url.pathname.split('/public/v1/minipos')[1],body=req.postDataJSON(),headers=req.headers();
  const lookup=req.method()==='GET'&&online&&suffix===`/orders/${ORDER.id}/operations/${online.unp_authority.operation_id}`;
  if(!lookup&&suffix!=='/operator-session'&&suffix!=='/execution-authorizations'&&suffix!=='/execution-authorizations:dispatch')return route.fallback();
  const call={method:req.method(),path:url.pathname,query:url.search,body,headers};calls.push(call);
  const app=headers['x-app-instance-id'];assert.ok(app);assert.equal(headers.authorization,'Bearer '+TEST_JWT);
  if(lookup){assert.equal(url.search,'');return route.fulfill({json:{operation_id:online.unp_authority.operation_id,client_operation_id:online.unp_authority.operation_id,state:resolved?'FISCALIZED':'UNKNOWN',...(resolved?{fiscal_reference:'RECOVERED-REF'}:{}),updated_at:new Date().toISOString()}})}
  if(suffix==='/operator-session'){assert.equal(req.method(),'GET');return route.fulfill({json:{...SESSION,session_id:sessionID}})}
  if(suffix==='/execution-authorizations'&&req.method()==='GET'){assert.equal(url.search,'?shift_id='+SHIFT.id);return route.fulfill({json:{tenant_id:'tenant-1',actor_subject:'subject-1',source_session_id:sessionID,app_instance_id:app}})}
  if(suffix==='/execution-authorizations'&&req.method()==='POST'){
   assert.equal(body.shift_id,SHIFT.id);assert.ok(headers['idempotency-key']);assert.ok(body.client_signing_public_key_der_base64);
   const dir=mkdtempSync(path.join(tmpdir(),'minipos-primary-authority-')),fixture=path.join(dir,'fixture.json'),plan=path.join(dir,'plan.json');
   if(body.order_id){assert.equal(body.order_id,ORDER.id);const row=base.ledger.find(r=>r.path.endsWith('/lines'));assert.ok(row);const {product_id,...line}=row.body as any;writeFileSync(plan,JSON.stringify({items:[line],payments:body.payments}));}
   execFileSync(oracle,['-test.run=^TestBrowserExecutionAuthorizationFixture$','-test.count=1'],{env:{...process.env,BROWSER_CLIENT_SPKI:body.client_signing_public_key_der_base64,BROWSER_APP_INSTANCE:app,BROWSER_AUTHORITY_FIXTURE:fixture,BROWSER_AUTHORITY_ONLINE:body.order_id?'1':'0',BROWSER_AUTHORITY_SOURCE_PLAN:body.order_id?plan:''},stdio:'pipe'});
   const value=JSON.parse(readFileSync(fixture,'utf8')),claims=JSON.parse(Buffer.from(value.authorization.payload,'base64url').toString());assert.equal(claims.client_signing_public_key_der_base64,body.client_signing_public_key_der_base64);assert.equal(claims.app_instance_id,app);issuances.push({request:call,fixture:value});if(body.order_id)online=claims;
   return route.fulfill({json:{authorization:value.authorization,...(body.order_id?{canonical_payload:value.command.canonical_payload}:{}),transport_token:'root-bound-transport',transport_token_expires_at:new Date(claims.exp*1000).toISOString()}});
  }
  assert.equal(suffix,'/execution-authorizations:dispatch');assert.equal(req.method(),'POST');assert.deepEqual(body,{order_id:ORDER.id});assert.ok(online?.unp_authority);assert.equal(headers['idempotency-key'],online.unp_authority.operation_id);dispatches++;
  return route.fulfill({json:{operation_id:online.unp_authority.operation_id,client_operation_id:online.unp_authority.operation_id,receipt_session_id:online.unp_authority.receipt_session_id,sale_id:online.unp_authority.server_sale_id,register_id:online.register_id,state:options.dispatchState??'FISCALIZED',...(options.dispatchState?{}:{fiscal_reference:'CLOUD-REF'}),updated_at:new Date().toISOString()}});
 });
 return{verifyNoSale(){assert.deepEqual(base.errors,[]);assert.ok(base.ledger.every(r=>r.known),'Unexpected API outside authority overlay');assert.equal(dispatches,0);assert.equal(issuances.filter(r=>r.request.body.order_id).length,0);assert.equal(base.ledger.filter(r=>r.method==='POST'&&(/\/orders(?:\/|$)/.test(r.path)||r.path.endsWith('/intents'))).length,0);},resolve(){resolved=true;},get ledger(){return [...base.ledger,...calls]},errors:base.errors,issuances,verify(){assert.deepEqual(base.errors,[]);assert.ok(base.ledger.every(r=>r.known));for(const suffix of ['/orders','/lines'])assert.equal(base.ledger.filter(r=>r.method==='POST'&&r.path.endsWith(suffix)).length,1);assert.equal(dispatches,1);assert.equal(issuances.filter(r=>r.request.body.order_id).length,1);assert.equal(base.ledger.filter(r=>r.path.endsWith('/checkout-batch')).length,0)}};
}
