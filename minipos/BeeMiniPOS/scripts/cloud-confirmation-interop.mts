import assert from 'node:assert/strict';
import {confirmCloudCheckout,type CloudCheckoutExpected} from '../src/cloudCheckoutConfirmation.ts';
const expected:CloudCheckoutExpected=JSON.parse(process.env.MINIPOS_CONFIRM_EXPECTED??'');
const calls:string[]=[];let removed=0;
const result=await confirmCloudCheckout({expected,live:()=>true,journal:{async remove(id){assert.equal(id,expected.sourceOrderID);removed++;}},read:async<T,>(path:string)=>{
 calls.push(path);
 const response=await fetch(process.env.MINIPOS_CONFIRM_URL+path,{headers:{Authorization:'Bearer '+process.env.MINIPOS_CONFIRM_TOKEN,'X-App-Instance-Id':'instance','X-Api-Version':'2026-08-07'}});
 const text=await response.text();assert.equal(response.status,200,'actor-scoped HTTP GET must succeed');
 const body=JSON.parse(text);
 if(!path.includes('/operations/')){
  for(const key of ['payments','fiscal_register_id','execution_authorization_id'])assert.equal(Object.hasOwn(body,key),false,'private field '+key);
  for(const secret of ['private-payment-one','private-payment-two','private-execution-authorization','4111111111111111','terminal-secret'])assert.equal(text.includes(secret),false,'private value leaked');
  assert.deepEqual(Object.keys(body.checkout_confirmation).sort(),['payments_sha256','root_register_id','version']);
 }
 return body as T;
}});
assert.equal(result.kind,process.env.MINIPOS_CONFIRM_KIND);assert.equal(removed,1);assert.equal(calls.length,2);assert.ok(calls[0].includes('/operations/'));assert.equal(calls[1],'/orders/'+expected.sourceOrderID);
console.log('actual HTTP → native confirmation PASS '+result.kind);
