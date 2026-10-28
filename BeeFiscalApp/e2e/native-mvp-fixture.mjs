import{startContractServer}from'./native-mvp-server.mjs';
import {createServer} from 'node:http';
import assert from 'node:assert/strict';
export const ids={register:'00000000-0000-4000-8000-000000000001',location:'00000000-0000-4000-8000-000000000002',device:'00000000-0000-4000-8000-000000000003',operator:'00000000-0000-4000-8000-000000000004'};
const tenant={tenant_id:'company-1',display_name:'Test Fiscal Company',roles:['ADMIN']};
const session={access_token:'mvp-access',refresh_token:'mvp-refresh',expires_in:3600,tenant};
export function fixtureContract(){let challenges=0,instance;const ledger=[];
 const respond=(method,rawURL,rawBody='',authorization='')=>{
 let status=200,out=null,body;try{body=rawBody?JSON.parse(rawBody):{}}catch{return {status:400,out:{code:'BAD_JSON'},accepted:false}}
 const url=new URL(rawURL,'http://fixture'),p=url.pathname.replace(/^\/public\/v1/,'');
 const exact=keys=>Object.keys(body).sort().join(',')===[...keys].sort().join(',');
 const validInstance=()=>typeof body.app_instance_id==='string'&&body.app_instance_id.length>5&&(!instance||instance===body.app_instance_id);
 if(method==='POST'&&p==='/app-auth/challenges'&&!url.search&&!authorization&&exact(['email','language','app_instance_id'])&&body.email==='qa@example.test'&&body.language==='en'&&validInstance()){instance=body.app_instance_id;challenges++;status=challenges===1?503:200;out=challenges===1?{code:'PRIVATE_BACKEND_FAILURE',details:{sql:'PRIVATE_SQL_TABLE'}}:{temporary_token:'mvp-temporary'}}
 else if(method==='POST'&&p==='/app-auth/challenges:verify'&&!url.search&&authorization==='Bearer mvp-temporary'&&exact(['code','app_instance_id'])&&validInstance()&&['000000','123456'].includes(body.code)){status=body.code==='000000'?400:200;out=status===400?{code:'PRIVATE_INVALID_OTP'}:{tenant_selection_token:'mvp-selection',tenants:[tenant]}}
 else if(method==='POST'&&p==='/app-auth/tenant-session'&&!url.search&&authorization==='Bearer mvp-selection'&&exact(['tenant_id','app_instance_id'])&&body.tenant_id===tenant.tenant_id&&validInstance())out=session;
 else if(method==='GET'&&p==='/app-auth/tenants'&&!url.search&&authorization==='Bearer mvp-access')out={items:[tenant]};
 else if(method==='POST'&&['/app-auth/refresh','/app-auth/logout'].includes(p)&&!url.search&&!authorization&&exact(['refresh_token','app_instance_id'])&&body.refresh_token==='mvp-refresh'&&validInstance())out=p.endsWith('/refresh')?session:{};
 else if(method==='GET'&&authorization==='Bearer mvp-access'){
  const paged=(items)=>({items,page:{has_more:false}}),query=url.searchParams;
  if(query.size===1&&query.get('limit')==='100'){
   if(p==='/locations')out=paged([{id:ids.location,code:'SOF',name:'Sofia'}]);
   if(p==='/registers')out=paged([{id:ids.register,code:'R01',location_id:ids.location,fiscal_device_id:ids.device,status:'ACTIVE',version:1}]);
   if(p==='/operators')out=paged([{id:ids.operator,operator_code:'A001',first_name:'Ivan',last_name:'Ivanov'}]);
   if(p==='/devices')out=paged([{id:ids.device,name:'Datecs counter',vendor:'Datecs',model:'DP-150 MX',status:'ACTIVE',transport:'RS232'}]);
  }
  if(!url.search){if(p===`/devices/${ids.device}/health`)out={effective_state:'READY',observed_at:'2026-10-20T00:00:00Z'};if(p===`/devices/${ids.device}/readiness`)out={ready:true,components:{fiscal_device:'READY'},recommended_transport:'REST'};if(p===`/devices/${ids.device}/activity`)out={items:[{state:'READY',at:'2026-10-20T00:00:00Z'}]};}
 }
 const accepted=out!==null;if(!accepted){status=501;out={code:'UNEXPECTED_REQUEST'}}
 ledger.push({method,url:rawURL,body,authorization,status,accepted});return {status,out,accepted};
 };
 return {ledger,respond};
}
export function verifyLedger(ledger){assert(ledger.length>0);assert(ledger.every(x=>x.accepted));const posts=ledger.filter(x=>x.method==='POST');assert.deepEqual(posts.map(x=>new URL(x.url,'http://fixture').pathname),['/public/v1/app-auth/challenges','/public/v1/app-auth/challenges','/public/v1/app-auth/challenges:verify','/public/v1/app-auth/challenges:verify','/public/v1/app-auth/tenant-session','/public/v1/app-auth/refresh','/public/v1/app-auth/logout']);assert.deepEqual(posts.map(x=>x.status),[503,200,400,200,200,200,200]);for(const p of ['/locations','/registers','/operators','/devices'])assert(ledger.some(x=>x.method==='GET'&&new URL(x.url,'http://fixture').pathname==='/public/v1'+p));assert(ledger.filter(x=>x.url==='/public/v1/app-auth/tenants').length===2);for(const suffix of ['readiness','health','activity'])assert(ledger.some(x=>x.url===`/public/v1/devices/${ids.device}/${suffix}`));return {status:'PASS_SCOPED_API',requests:ledger.length,posts:posts.length,physicalWrites:0};}
export async function startFixture(port){const f=fixtureContract();return {...f,server:await startContractServer(port,f)};}
