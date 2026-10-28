/** Independent replay of this presentation fixture's actual MiniPOS API contract.
 * No acceptance of unknown endpoints, caller identities, or additional payload properties. */
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
export const fixtureActor='Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0ZW5hbnRfaWQiOiIwMDAwMDAwMC0wMDAwLTQwMDAtODAwMC0wMDAwMDAwMDAwMDEiLCJzdWIiOiJ1c2VyLTEiLCJleHAiOjk5OTk5OTk5OTl9.e2e_test';
const uuid=value=>assert.match(value,/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i);
const keys=(value,names)=>assert.deepEqual(Object.keys(value).sort(),names.sort());
export function verifyResultLedger(calls,caller){
 assert.ok(['printer','sale','reverse','uncertain','rejected'].includes(caller));assert.ok(calls.length);
 for(const row of calls){assert.equal(row.known,true);assert.equal(row.status,200);assert.equal(row.actor,row.path==='/beeloy/local/v1/healthz'?null:row.path.startsWith('/beeloy/local/')?'Bearer local-adapter-token':fixtureActor);
 if(row.method==='GET'){assert.equal(row.body,null);assert.ok(['/public/v1/minipos/operator-session','/public/v1/minipos/configuration','/public/v1/minipos/products','/public/v1/minipos/shifts','/public/v1/minipos/fiscal-route-health','/beeloy/local/v1/healthz'].includes(row.path));assert.equal(row.query,row.path.endsWith('/shifts')?'?employee_id=emp-001&state=OPEN':'');continue}
 assert.equal(row.method,'POST');assert.equal(row.query,'');const b=row.body;assert.ok(b&&typeof b==='object'&&!Array.isArray(b));if(!row.path.endsWith('/orders'))uuid(row.key);
 switch(row.path){
 case'/public/v1/minipos/fiscal-local-tokens':assert.deepEqual(b,{adapter_device_id:'adapter-001',location_id:'loc-001',register_id:'reg-001',operator_id:'emp-001',shift_id:'shift-001',binding_generation:1,adapter_base_url:'http://localhost:19001/beeloy/local/v1'});break;
 case'/public/v1/minipos/orders':assert.equal(row.key,null);assert.deepEqual(b,{shift_id:'shift-001'});break;
 case'/public/v1/minipos/orders/order-001/lines':keys(b,['line_id','product_id','name','quantity','unit_price','tax_group']);uuid(b.line_id);assert.deepEqual({...b,line_id:'random'},{line_id:'random',product_id:'prod-001',name:'Кафе',quantity:'1.000',unit_price:{amount:'2.50',currency:'EUR'},tax_group:'B'});assert.equal(row.ifMatch,'"1"');break;
 case'/public/v1/minipos/orders/order-001/checkout-batch':keys(b,['payments','metadata']);assert.equal(b.payments.length,1);keys(b.payments[0],['payment_id','type','amount']);uuid(b.payments[0].payment_id);assert.deepEqual({...b.payments[0],payment_id:'random'},{payment_id:'random',type:'CASH',amount:{amount:'2.50',currency:'EUR'}});keys(b.metadata,['receipt_session_id']);uuid(b.metadata.receipt_session_id);break;
 case'/public/v1/minipos/orders/order-001/reversals':assert.deepEqual(b,{reason_code:'CUSTOMER_RETURN'});break;
 case'/beeloy/local/v1/intents':keys(b,['protocol_version','intent_id','action','client_operation_id','receipt_session_id','client_sale_surrogate_id','server_sale_id','operator_code','tenant_id','location_id','register_id','edge_device_id','binding_generation','command','items','payments','metadata','canonical_payload','payload_sha256']);uuid(b.receipt_session_id);assert.deepEqual({...b,receipt_session_id:'random'},{protocol_version:'1.0',intent_id:row.key,action:'PRINTER_TEST',client_operation_id:row.key,receipt_session_id:'random',client_sale_surrogate_id:row.key,server_sale_id:null,operator_code:'001',tenant_id:'00000000-0000-4000-8000-000000000001',location_id:'loc-001',register_id:'reg-001',edge_device_id:'adapter-001',binding_generation:1,command:'PRINTER_TEST',items:[],payments:[],metadata:{},canonical_payload:{metadata:{}},payload_sha256:createHash('sha256').update('{"metadata":{}}').digest('hex')});break;
 default:assert.fail('Unknown API request '+row.path);
 }}
 const writes=calls.filter(r=>r.method==='POST'),count=suffix=>writes.filter(r=>r.path.endsWith(suffix)).length;assert.equal(count('/fiscal-local-tokens'),1);if(['printer','uncertain','rejected'].includes(caller)){assert.equal(count('/intents'),1);assert.equal(writes.length,2)}else{assert.equal(count('/orders'),1);assert.equal(count('/lines'),1);assert.equal(count('/checkout-batch'),1);assert.equal(count('/reversals'),caller==='reverse'?1:0);assert.equal(writes.length,caller==='reverse'?5:4)}
}
