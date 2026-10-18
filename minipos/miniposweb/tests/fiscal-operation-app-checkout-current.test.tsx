// @vitest-environment jsdom
vi.mock('../src/ui/useHeaderLeaveGuard',()=>({useHeaderLeaveGuard:()=>({register:()=>()=>{},request:(go:()=>void)=>go(),visible:false,isBlocked:()=>false,canEdit:()=>true}),HeaderLeaveDialog:()=>null}));
vi.mock('../src/ui/PaperOperationalHeader',()=>({PaperOperationalHeader:()=>null}));
import React,{act}from'react';import{createRoot}from'react-dom/client';import{it,expect,vi}from'vitest';import{webcrypto,createPublicKey,verify}from'node:crypto';
import {IDBFactory} from 'fake-indexeddb';
import {execFileSync} from 'node:child_process';import fs from 'node:fs';import os from 'node:os';import path from 'node:path';
const s=vi.hoisted(()=>({grid:null as any,payment:null as any,intent:null as any,dispatch:vi.fn(),reverse:vi.fn(),line:vi.fn(),authorize:vi.fn(),envelope:null as any}));
vi.mock('react-native',()=>({Platform:{OS:'web'},View:({children}:any)=><div>{children}</div>}));
vi.mock('react-native-paper',()=>{const C=({children,testID}:any)=><div data-testid={testID}>{children}</div>;return{Text:C,Surface:C,Card:Object.assign(C,{Content:C}),HelperText:C,ActivityIndicator:C,useTheme:()=>({colors:{background:'#fff'}})}});
vi.mock('../src/ui/app-frame',()=>({AppFrame:({children}:any)=><div>{children}</div>}));vi.mock('../src/ui/app-button',()=>({AppButton:({children,onPress,testID}:any)=><button data-testid={testID} onClick={onPress}>{children}</button>}));
vi.mock('../src/ui/PreferenceControls',()=>({readLanguage:()=> 'en',PreferenceControls:()=>null}));vi.mock('../src/AuthFlow',()=>({AuthFlow:()=>null}));vi.mock('../src/Management',()=>({Management:()=>null}));vi.mock('../src/OutboxPanel',()=>({OutboxPanel:()=>null}));vi.mock('../src/ui/FiscalOperationResult',()=>({FiscalOperationResult:()=>null}));
vi.mock('../src/SalePanels',()=>({ProductGrid:(p:any)=>{s.grid=p;return<div/>},PaymentPanel:(p:any)=>{s.payment=p;return<div/>}}));
vi.mock('../src/storage/referenceCache',()=>({cacheGet:async()=>undefined,cachePut:async()=>{}}));vi.mock('../src/storage/outbox',()=>({getOutbox:async()=>undefined,listOutbox:async()=>[],putOutbox:async()=>{}}));vi.mock('../src/fiscal/cloudRecovery',()=>({refreshCloudOutbox:async()=>{}}));vi.mock('../src/fiscal/webBleRoute',()=>({WebBleRoute:class{}}));
vi.mock('../src/fiscal/routeController',()=>({RouteController:class{constructor(public routes:any[]){}async execute(intent:any){s.intent=intent;return this.routes[0].submit(intent,new AbortController().signal)}}}));
vi.mock('../src/api/client',async()=>{const actual=await vi.importActual<any>('../src/api/client');return{...actual,EmailAuthClient:class{},LocalFiscalClient:class{},MiniPosClient:class{
 constructor(readonly baseUrl:string,readonly token:()=>string,readonly appInstance:string){}
 session=async()=>({authenticated:true,session_id:'authenticated-source-session-0000000000000001',employee:{id:'source-employee',operator_code:'A001',first_name:'A',last_name:'B'},expires_at:'2099-01-01'});
 configuration=async()=>({location_id:'source-location',fiscal_register_id:'source-register',fiscal_adapter_id:'source-adapter',binding_generation:99,adapter_base_url:'http://localhost'});
 products=async()=>({items:[{id:'product',status:'ACTIVE',name:'Coffee',price:{amount:'2.50',currency:'EUR'},tax_group:'B'}]});shifts=async()=>({items:[{id:'source-shift'}]});localToken=async()=>({access_token:'legacy-read-token',expires_at:'2099-01-01'});
 executionIdentity=async()=>({tenant_id:'tenant-1',actor_subject:'subject-1',source_session_id:'authenticated-source-session-0000000000000001',app_instance_id:this.appInstance});
 executionAuthorization=async(input:any)=>{s.authorize(input);const dir=fs.mkdtempSync(path.join(os.tmpdir(),'app-root-authority-')),fixture=path.join(dir,'fixture.json'),plan=path.join(dir,'plan.json');
 if(input.order_id){const source=s.line.mock.calls.at(-1)?.[2];if(!source)throw new Error("Persisted source line required before authorization");const{product_id,...line}=source;fs.writeFileSync(plan,JSON.stringify({items:[line],payments:input.payments}));}
 execFileSync('go',['test','./internal/domain','-run','^TestBrowserExecutionAuthorizationFixture$','-count=1'],{cwd:path.resolve('../../fiscal-backend'),env:{...process.env,GOCACHE:'/private/tmp/fiscal-web-authority-go-cache',BROWSER_CLIENT_SPKI:input.client_signing_public_key_der_base64,BROWSER_APP_INSTANCE:this.appInstance,BROWSER_AUTHORITY_FIXTURE:fixture,BROWSER_AUTHORITY_ONLINE:input.order_id?'1':'0',BROWSER_AUTHORITY_SOURCE_PLAN:input.order_id?plan:''},stdio:'pipe'});
 const result=JSON.parse(fs.readFileSync(fixture,'utf8'));s.envelope=result.authorization;return{authorization:result.authorization,canonical_payload:input.order_id?JSON.parse(Buffer.from(result.authorization.payload,'base64url').toString()).unp_authority&&result.command.canonical_payload:undefined,transport_token:'root-bound-transport'};};
 createOrder=async()=>({id:'source-order',external_id:'source-external',version:1});addLine=async(...args:any[])=>{s.line(...args);return{version:2}};
 dispatchExecution=async(...args:any[])=>{s.dispatch(...args);return{operation_id:args[1].intent_id,state:'FISCALIZED',certainty:'PROVEN_SUCCESS'}};
 reverseOrder=async(...args:any[])=>{s.reverse(...args);return{operation_id:'reverse',state:'FISCALIZED'}};
}}});
import{canonical}from'../src/fiscal/intentBuilder';
for(const discount of[undefined,0,0.2])it(`actual App checkout discount ${discount} prepares root authority and signs exact money intent then reverses`,async()=>{
 (globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;Object.defineProperty(globalThis,'crypto',{configurable:true,value:webcrypto});Object.defineProperty(globalThis,'indexedDB',{configurable:true,value:new IDBFactory()});localStorage.clear();sessionStorage.clear();localStorage.setItem('minipos-access-token','x.'+btoa(JSON.stringify({tenant_id:'source-tenant'}))+'.x');
 s.dispatch.mockClear();s.reverse.mockClear();s.line.mockClear();s.authorize.mockClear();s.payment=null;s.grid=null;
 const{default:App}=await import('../src/App');const host=document.createElement('div'),root=createRoot(host);
 try{
  await act(async()=>root.render(<App/>));expect(s.grid).not.toBeNull();
  // Real manager, IDB key storage and actual Go issuer are allowed to finish.
  await act(async()=>{await vi.waitFor(()=>expect(s.authorize).toHaveBeenCalled(),{timeout:15000});});
  await act(async()=>s.grid.onAdd(s.grid.products[0]));if(discount!==undefined)await act(async()=>s.payment.onUpdate('product','discount',discount));
  await act(async()=>s.payment.onCheckout());expect(s.dispatch).toHaveBeenCalledTimes(1);
  const item=s.intent.items[0],line=s.intent.canonical_payload.items[0];
  if(discount&&discount>0){expect(item.discount).toEqual({amount:'0.20',currency:'EUR'});expect(line.discount).toEqual(item.discount)}else{expect(Object.hasOwn(item,'discount')).toBe(false);expect(Object.hasOwn(line,'discount')).toBe(false)}
  expect(s.intent.canonical_payload).toEqual({currency:'EUR',server_sale_id:'online-sale',external_id:'offline-external-1',location_id:'location-1',operator_id:'operator-1',operator_code:'A001',items:[{line_id:item.item_id,name:'Coffee',quantity:'1.000',unit_price:{amount:'2.50',currency:'EUR'},tax_group:'B',...(discount&&discount>0?{discount:{amount:'0.20',currency:'EUR'}}:{})}],payments:[{payment_id:s.intent.payments[0].payment_id,type:'CASH',amount:{amount:discount&&discount>0?'2.30':'2.50',currency:'EUR'}}],metadata:{},unp_source:'SERVER_ASSIGNED',unp:'AB123456-A001-0005000'});
  expect(s.intent.server_sale_id).toBe('online-sale');expect(s.intent.server_sale_id).not.toBe('source-order');expect(s.intent.tenant_id).toBe('tenant-1');expect(s.intent.register_id).toBe('register-1');expect(s.intent.binding_generation).toBe(7);
  const claims=JSON.parse(Buffer.from(s.intent.execution_authorization.payload,'base64url').toString());expect(claims.unp_authority.payload_sha256).toBe(s.intent.payload_sha256);expect(claims.unp_authority.operation_id).toBe(s.intent.intent_id);
  const proof={kind:'EXECUTION_INTENT_PROOF',version:1,authorization_sha256:s.intent.execution_authorization.sha256,operation_id:s.intent.intent_id,receipt_session_id:s.intent.receipt_session_id,payload_sha256:s.intent.payload_sha256,server_sale_id:s.intent.server_sale_id,client_sale_surrogate_id:s.intent.client_sale_surrogate_id,binding_sha256:claims.binding_sha256,binding_generation:7,command_type:'SALE_FINALIZE'};
  const key=createPublicKey({key:Buffer.from(claims.client_signing_public_key_der_base64,'base64url'),format:'der',type:'spki'});expect(verify('sha256',Buffer.from('BEELOY-EXECUTION-INTENT-PROOF-V1\n'+canonical(proof)),key,Buffer.from(s.intent.client_proof.signature,'base64url'))).toBe(true);
  expect(s.intent.payload_sha256).toMatch(/^[0-9a-f]{64}$/);expect(()=>canonical(s.intent.canonical_payload)).not.toThrow();expect(s.line).toHaveBeenCalledTimes(1);
  if(discount&&discount>0)expect(s.line.mock.calls[0][2].discount).toEqual({amount:'0.20',currency:'EUR'});else expect(JSON.stringify(s.line.mock.calls[0][2])).not.toContain('discount');
  expect(s.dispatch.mock.calls[0][0]).toBe('source-order');expect(s.dispatch.mock.calls[0][1]).toBe(s.intent);expect(s.payment.lastOrderId).toBe('source-order');
  await act(async()=>s.payment.onReverse());expect(s.reverse).toHaveBeenCalledWith('source-order','CUSTOMER_RETURN',expect.any(String));
 }finally{await act(async()=>root.unmount())}
},30000);
