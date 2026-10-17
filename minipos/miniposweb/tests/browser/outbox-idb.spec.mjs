import { test, expect } from '@playwright/test';
const row = {intent:{client_operation_id:'one',payload_sha256:'hash',tenant_id:'t1',location_id:'l1',register_id:'r1',operator_code:'op1',edge_device_id:'e1',binding_generation:1},createdAt:'initial',updatedAt:'initial',posSync:{shift_id:'shift',external_id:'ext',lines:[],payments:[]}};
const result={operation_id:'one',state:'COMMITTED',certainty:'PROVEN_SUCCESS',updated_at:'now'};
test.beforeEach(async({page})=>{await page.goto('/');});
test('real IndexedDB serializes concurrent prepare and cross-tab claims',async({page,context})=>{
 await page.evaluate(async row=>{const m=await import('/src/storage/outbox');const rows=await Promise.all([m.prepareOutbox(row),m.prepareOutbox({...row,posSync:{...row.posSync,shift_id:'must not overwrite'}})]);if(rows[1].posSync.shift_id!=='shift')throw Error('overwrite');},row);
 const other=await context.newPage();await other.goto('/');
 const claims=await Promise.all([page.evaluate(async row=>(await import('/src/storage/outbox')).claimOutboxAttempt(row,'CLOUD'),row),other.evaluate(async row=>(await import('/src/storage/outbox')).claimOutboxAttempt(row,'BLE'),row)]);
 expect(claims.filter(Boolean)).toHaveLength(1);
 expect(await page.evaluate(async()=>(await import('/src/storage/outbox')).getOutbox('one'))).toMatchObject({posSync:row.posSync,createdAt:'initial',intent:row.intent});
});
test('aborted claim rolls back and prevents submit',async({page})=>{
 const actual=await page.evaluate(async row=>{const m=await import('/src/storage/outbox');const {RouteController}=await import('/src/fiscal/routeController');const original=IDBObjectStore.prototype.put;IDBObjectStore.prototype.put=function(value,...args){const request=original.call(this,value,...args);if(value.attempt)this.transaction.abort();return request;};let submitted=0;let failed=false;try{await new RouteController([{kind:'CLOUD',probe:async()=>true,lookup:async()=>{throw Error('unused');},submit:async()=>{submitted++;throw Error('unexpected');}}]).execute(row.intent);}catch{failed=true;}finally{IDBObjectStore.prototype.put=original;}return {failed,submitted,saved:await m.getOutbox('one')};},row);
 expect(actual.failed).toBe(true);expect(actual.submitted).toBe(0);expect(actual.saved.attempt).toBeUndefined();
});
test('slow result cannot overwrite changed scope',async({page})=>{
 const actual=await page.evaluate(async({row,result})=>{const m=await import('/src/storage/outbox');await m.prepareOutbox(row);await m.claimOutboxAttempt(row,'CLOUD');await new Promise((resolve,reject)=>{const open=indexedDB.open('beeloy-miniposweb',1);open.onsuccess=()=>{const db=open.result,tx=db.transaction('fiscal-outbox','readwrite');tx.objectStore('fiscal-outbox').put({...row,intent:{...row.intent,tenant_id:'new-tenant'}});tx.oncomplete=()=>{db.close();resolve();};tx.onerror=()=>reject(tx.error);};});let error='';try{await m.updateOutboxResult(row.intent,result);}catch(e){error=e.message;}return {error,saved:await m.getOutbox('one')};},{row,result});
 expect(actual.error).toBe('OUTBOX_SCOPE_CONFLICT');expect(actual.saved.intent.tenant_id).toBe('new-tenant');expect(actual.saved.result).toBeUndefined();
});
test('result merge preserves latest posSync after await',async({page})=>{
 const actual=await page.evaluate(async({row,result})=>{const m=await import('/src/storage/outbox');await m.prepareOutbox(row);await m.claimOutboxAttempt(row,'CLOUD');const stale=await m.getOutbox('one');await m.putOutbox({...stale,posSync:{...stale.posSync,synced_at:'newly-synced'}});await m.updateOutboxResult(row.intent,result);return m.getOutbox('one');},{row,result});
 expect(actual.posSync.synced_at).toBe('newly-synced');expect(actual.attempt.route).toBe('CLOUD');expect(actual.result).toEqual(result);
});
test('actual IndexedDB digest/scope conflicts cannot claim or reinitialize',async({page})=>{
 const errors=await page.evaluate(async row=>{const m=await import('/src/storage/outbox');await m.prepareOutbox(row);const errors=[];for(const changed of [{...row,intent:{...row.intent,payload_sha256:'other'}},{...row,intent:{...row.intent,operator_code:'other'}}])for(const fn of [()=>m.prepareOutbox(changed),()=>m.claimOutboxAttempt(changed,'BLE')])try{await fn();errors.push('unexpected');}catch(e){errors.push(e.message);}return errors;},row);
 expect(errors).toEqual(['IDEMPOTENCY_PAYLOAD_CONFLICT','IDEMPOTENCY_PAYLOAD_CONFLICT','OUTBOX_SCOPE_CONFLICT','OUTBOX_SCOPE_CONFLICT']);
});
test('response lost then reload cannot resubmit through cloud or alternate BLE',async({page})=>{
 let submitted=0;await page.exposeFunction('recordSubmit',()=>{submitted++;});
 await page.evaluate(async row=>{const {RouteController}=await import('/src/fiscal/routeController');try{await new RouteController([{kind:'CLOUD',probe:async()=>true,lookup:async()=>{throw Error('unused');},submit:async()=>{await window.recordSubmit();throw Error('lost response');}}]).execute(row.intent);}catch{}},row);
 await page.reload();
 for(const status of [401,503,404])await page.evaluate(async({row,status})=>{const {RouteController}=await import('/src/fiscal/routeController');const route=kind=>({kind,probe:async()=>true,lookup:async()=>{throw {status};},submit:async()=>{await window.recordSubmit();throw Error('duplicate');}});try{await new RouteController([route('CLOUD'),route('BLE')]).execute(row.intent);}catch{}},{row,status});
 expect(submitted).toBe(1);
});

test('stale UNKNOWN result cannot overwrite proven terminal via update or generic put',async({page})=>{
 const actual=await page.evaluate(async({row,result})=>{const m=await import('/src/storage/outbox');await m.prepareOutbox(row);await m.claimOutboxAttempt(row,'CLOUD');const stale=await m.getOutbox('one');const unknown={...result,state:'UNKNOWN',certainty:'UNKNOWN'};const outcomes=await Promise.all([m.updateOutboxResult(row.intent,result),m.updateOutboxResult(row.intent,unknown)]);await m.putOutbox({...stale,result:unknown});return {outcomes,saved:await m.getOutbox('one')};},{row,result});
 expect(actual.outcomes[1]).toEqual(result);expect(actual.saved.result).toEqual(result);
});
test('conflicting proven terminal outcomes fail closed and retain original',async({page})=>{
 const actual=await page.evaluate(async({row,result})=>{const m=await import('/src/storage/outbox');await m.prepareOutbox(row);await m.updateOutboxResult(row.intent,result);const changed={...result,fiscal_reference:'conflict'};const errors=[];for(const fn of [()=>m.updateOutboxResult(row.intent,changed),()=>m.putOutbox({...row,result:changed})])try{await fn();errors.push('unexpected');}catch(e){errors.push(e.message);}return {errors,saved:await m.getOutbox('one')};},{row,result});
 expect(actual.errors).toEqual(['OUTBOX_TERMINAL_RESULT_CONFLICT','OUTBOX_TERMINAL_RESULT_CONFLICT']);expect(actual.saved.result).toEqual(result);
});
test('generic caller cannot erase attempt, latest sync marker or change ownership',async({page})=>{
 const actual=await page.evaluate(async row=>{const m=await import('/src/storage/outbox');await m.prepareOutbox(row);await m.claimOutboxAttempt(row,'CLOUD');const claimed=await m.getOutbox('one');await m.putOutbox({...claimed,posSync:{...claimed.posSync,synced_at:'synced'}});await m.putOutbox(row);let error='';try{await m.putOutbox({...row,intent:{...row.intent,tenant_id:'other'}});}catch(e){error=e.message;}return {error,saved:await m.getOutbox('one')};},row);
 expect(actual.saved.attempt.route).toBe('CLOUD');expect(actual.saved.posSync.synced_at).toBe('synced');expect(actual.saved.intent).toEqual(row.intent);expect(actual.error).toBe('OUTBOX_SCOPE_CONFLICT');
});

test('same UUID and payload digest cannot change command or receipt session',async({page})=>{
 const actual=await page.evaluate(async row=>{const m=await import('/src/storage/outbox');const original={...row,intent:{...row.intent,action:'REPORT_X',command:'REPORT_X',receipt_session_id:'receipt1'}};await m.prepareOutbox(original);const errors=[];for(const key of ['action','command','receipt_session_id','intent_id','protocol_version','server_sale_id','client_sale_surrogate_id']){const changed={...original,intent:{...original.intent,[key]:'changed'}};for(const fn of [()=>m.prepareOutbox(changed),()=>m.claimOutboxAttempt(changed,'BLE'),()=>m.putOutbox(changed)])try{await fn();errors.push('unexpected');}catch(e){errors.push(e.message);}}return {errors,saved:await m.getOutbox('one')};},row);
 expect(actual.errors).toHaveLength(21);expect(actual.errors.every(value=>value==='IDEMPOTENCY_ENVELOPE_CONFLICT')).toBe(true);expect(actual.saved.intent.command).toBe('REPORT_X');expect(actual.saved.intent.receipt_session_id).toBe('receipt1');expect(actual.saved.attempt).toBeUndefined();
});

test('actual cloud client recovers lost response after reload into real IndexedDB with GET only',async({page})=>{
 const original={...row,intent:{...row.intent,metadata:{order_id:'order-1'}}};
 let submissions=0;await page.exposeFunction('recordSubmit',()=>{submissions++;});
 await page.evaluate(async row=>{const {RouteController}=await import('/src/fiscal/routeController');try{await new RouteController([{kind:'CLOUD',probe:async()=>true,lookup:async()=>{throw Error('unused');},submit:async()=>{await window.recordSubmit();throw Error('response lost after acceptance');}}]).execute(row.intent);}catch{}const m=await import('/src/storage/outbox');const saved=await m.getOutbox(row.intent.client_operation_id);if(saved.attempt.route!=='CLOUD')throw Error('attempt not durable');},original);
 await page.reload();const requests=[];
 await page.route('**/minipos/orders/**',async route=>{requests.push({method:route.request().method(),url:route.request().url(),headers:route.request().headers()});await route.fulfill({status:200,json:{operation_id:'one',client_operation_id:'one',state:'FISCALIZED',fiscal_reference:'device-receipt',updated_at:'2026-10-17T20:00:00Z'}});});
 const saved=await page.evaluate(async()=>{const {MiniPosClient}=await import('/src/api/client');const {refreshCloudOutbox}=await import('/src/fiscal/cloudRecovery');const rows=await refreshCloudOutbox(new MiniPosClient('/minipos',()=> 'employee-token','app-instance'),{tenant:'t1',location:'l1',register:'r1',operator:'op1'},()=>true);return rows[0];});
 expect(submissions).toBe(1);expect(requests).toHaveLength(1);expect(requests[0]).toMatchObject({method:'GET',headers:{authorization:'Bearer employee-token','x-app-instance-id':'app-instance','x-api-version':'2026-08-07'}});
 expect(saved.attempt.route).toBe('CLOUD');expect(saved.result).toMatchObject({operation_id:'one',state:'FISCALIZED',certainty:'PROVEN_SUCCESS',fiscal_reference:'device-receipt'});
 await page.reload();expect(await page.evaluate(async()=>(await import('/src/storage/outbox')).getOutbox('one'))).toMatchObject({result:saved.result,attempt:saved.attempt});
});

test('actual cloud errors and mismatched proof retain uncertain claim across reload',async({page})=>{
 const original={...row,intent:{...row.intent,metadata:{order_id:'order-1'}}};
 await page.evaluate(async row=>{const m=await import('/src/storage/outbox');await m.prepareOutbox(row);await m.claimOutboxAttempt(row,'CLOUD');},original);
 let status=401;let mismatched=false;const methods=[];
 await page.route('**/minipos/orders/**',async route=>{methods.push(route.request().method());await route.fulfill({status,json:mismatched?{operation_id:'foreign',client_operation_id:'one',state:'FISCALIZED',fiscal_reference:'foreign-proof'}:{error:'uncertain'}});});
 for(const next of [401,403,404,409,503,200]){status=next;mismatched=next===200;await page.reload();const saved=await page.evaluate(async()=>{const {MiniPosClient}=await import('/src/api/client');const {refreshCloudOutbox}=await import('/src/fiscal/cloudRecovery');return (await refreshCloudOutbox(new MiniPosClient('/minipos',()=> 'token','instance'),{tenant:'t1',location:'l1',register:'r1',operator:'op1'},()=>true))[0];});expect(saved.attempt.route).toBe('CLOUD');expect(saved.result).toBeUndefined();}
 expect(methods).toEqual(Array(6).fill('GET'));
});
