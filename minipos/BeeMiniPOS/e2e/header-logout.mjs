import {expect} from '@playwright/test';import assert from 'node:assert/strict';
/** Ordinary production OTP UI with fixture-owned operator; never seeds session storage. */
export async function assertDirtyOperatorLogout(browser,appUrl){
 const page=await browser.newPage(),wire=[],unknown=[];const token='header-operator-fixture-token';
 const employee={id:'00000000-0000-4000-8000-000000000004',first_name:'Test',last_name:'Admin',operator_code:'0001',roles:['ADMIN']};
 const respond=(route,body,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
 await page.route('http://minipos-api.test/**',async route=>{
  const request=route.request(),url=new URL(request.url()),method=request.method(),body=request.postData()?request.postDataJSON():null;
  const path=url.pathname,list=['/public/v1/minipos/products','/public/v1/minipos/employees','/public/v1/minipos/tax-groups'].includes(path);if(url.search!==(list?'?limit=100':'')){unknown.push({path,method,reason:'query'});return respond(route,{code:'UNEXPECTED_QUERY'},500)}
  wire.push({path,method,query:url.search,body,authorization:request.headers().authorization??null});
  if(path==='/public/v1/minipos/auth/request-code'&&method==='POST'){assert.deepEqual(body,{email:'manager@example.test',language:'bg'});assert.equal(request.headers().authorization,undefined);return respond(route,{})}
  if(path==='/public/v1/minipos/auth/verify-code'&&method==='POST'){assert.deepEqual(body,{email:'manager@example.test',code:'123456'});return respond(route,{access_token:token,refresh_token:'header-refresh',expires_in:3600})}
  assert.equal(request.headers().authorization,`Bearer ${token}`);if(method==='GET')assert.equal(body,null);
  if(path==='/public/v1/minipos/operator-session'&&method==='GET')return respond(route,{authenticated:true,employee,expires_at:'2099-01-01T00:00:00Z'});
  if(path==='/public/v1/minipos/operator-session'&&method==='POST'){assert.deepEqual(body,{});return respond(route,{})}
  if(path==='/public/v1/minipos/products'&&method==='GET')return respond(route,{items:[]});
  if(path==='/public/v1/minipos/tax-groups'&&method==='GET')return respond(route,{items:[{id:'tax-b',code:'B',name:'Standard',rate:'20.00',status:'ACTIVE',version:1}]});
  if(path==='/public/v1/minipos/employees'&&method==='GET')return respond(route,{items:[employee]});
  if(path==='/public/v1/minipos/configuration'&&method==='GET')return respond(route,{code:'NOT_FOUND'},404);
  unknown.push({path,method});return respond(route,{code:'UNEXPECTED'},500);
 });
 await page.route('http://fiscal-api.test/**',route=>{const request=route.request(),url=new URL(request.url());if(request.method()==='GET'&&url.pathname==='/public/v1/registers'&&url.search==='?limit=100'&&request.postData()===null&&request.headers().authorization===`Bearer ${token}`)return respond(route,{items:[]});unknown.push({path:url.pathname,method:request.method()});return respond(route,{code:'UNEXPECTED_FISCAL'},500)});
 try{
  await page.goto(appUrl);await page.getByTestId('operator-login-email').fill('manager@example.test');await page.getByTestId('operator-login-start').click();await page.getByTestId('operator-login-code').fill('123456');await page.getByTestId('operator-login-verify').click();await page.getByTestId('admin-editors').waitFor();await page.getByTestId('admin-products').click();await page.getByTestId('product-add').click();await page.getByTestId('product-name').fill('Unsaved logout draft');
  const logoutWrites=()=>wire.filter(row=>row.path==='/public/v1/minipos/operator-session'&&row.method==='POST');
  await page.getByTestId('operator-logout').click();await expect(page.getByTestId('header.leave.dialog')).toBeVisible();await page.getByTestId('header.leave.stay').click();expect(logoutWrites()).toHaveLength(0);await expect(page.getByTestId('product-name')).toHaveValue('Unsaved logout draft');
  await page.getByTestId('operator-logout').click();await page.getByTestId('header.leave.discard').click();await page.getByTestId('operator-login-email').waitFor();expect(logoutWrites()).toHaveLength(1);expect(wire.filter(row=>row.method==='POST'&&!row.path.includes('/auth/')&&row.path!=='/public/v1/minipos/operator-session')).toEqual([]);expect(unknown).toEqual([]);for(const path of ['/auth/request-code','/auth/verify-code'])expect(wire.filter(row=>row.path==='/public/v1/minipos'+path)).toHaveLength(1);for(const path of ['/operator-session','/products','/tax-groups','/employees','/configuration'])expect(wire.filter(row=>row.path==='/public/v1/minipos'+path&&row.method==='GET')).toHaveLength(1);
 }finally{await page.close()}
}
