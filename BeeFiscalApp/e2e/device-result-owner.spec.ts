import{test,expect}from'@playwright/test';
import{mockFiscalApi,openApp,DEVICE_ID,SECOND_DEVICE_ID,REGISTER_ID}from'./helpers';
const API='http://fiscal-admin.test/public/v1';
for(const [size,width,height] of [['xs',320,900],['md',768,1000],['lg',1440,1000]] as const)for(const mode of ['light','dark'] as const){
 test(`Completed device results retire on register A-B-A ${size}/${mode}`,async({page},info)=>{
  await page.setViewportSize({width,height});await page.addInitScript(mode=>localStorage.setItem('beefiscal.ui.preferences.v1',JSON.stringify({language:'en',themeMode:mode})),mode);
  const errors:string[]=[],bad:string[]=[];page.on('pageerror',e=>errors.push(e.message));page.on('response',r=>{if(r.url().startsWith(API)&&r.status()>=400)bad.push(r.status()+' '+r.url())});
  const mutations=await mockFiscalApi(page,{secondRegister:true});let probes=0,diagnostics=0;
  await page.route(`${API}/registers/${REGISTER_ID}/connectivity-probes`,async r=>{expect(r.request().method()).toBe('POST');expect(r.request().postDataJSON()).toEqual({});probes++;await r.fulfill({contentType:'application/json',body:JSON.stringify({state:'OLD_REGISTER_PROBE',hops:{cloud:{state:'READY'}},recommended_transport:'REST'})})});
  await page.route(`${API}/devices/${DEVICE_ID}/diagnostics`,async r=>{diagnostics++;await r.fulfill({contentType:'application/json',body:JSON.stringify({device_id:DEVICE_ID,observed_at:'2026-10-18T00:00:00Z',metrics:{owner:'OLD_DEVICE_DIAGNOSTIC'},redactions_applied:true})})});
  await openApp(page);await page.getByTestId('readiness-probe').click();await expect(page.getByTestId('transport-metrics')).toContainText('OLD_REGISTER_PROBE');await expect(page.getByTestId('fiscal-device-metrics')).toBeVisible();
  await page.getByTestId('device-diagnostics').click();await expect(page.getByTestId('diagnostics-result')).toContainText('OLD_DEVICE_DIAGNOSTIC');
  await page.getByTestId('smart-device-activation-code').fill('BC1234');await page.getByTestId('smart-device-activation-lookup').click();await expect(page.getByTestId('smart-device-activation-preview')).toContainText('BC-001');
  for(const register of ['R02','R01']){await page.getByRole('radio',{name:register,exact:true}).first().click();await expect(page.getByTestId('device.item.'+(register==='R02'?SECOND_DEVICE_ID:DEVICE_ID))).toBeVisible();await expect(page.getByTestId('screen-loading')).toHaveCount(0);for(const id of ['transport-metrics','fiscal-device-metrics','diagnostics-result','smart-device-activation-preview','provisioning-result','ble-session-result','bluecash-activation-result'])await expect(page.getByTestId(id)).toHaveCount(0);}
  expect(probes).toBe(1);expect(diagnostics).toBe(1);expect(mutations).toEqual([]);expect(errors).toEqual([]);expect(bad).toEqual([]);await info.attach('owner-proof',{body:JSON.stringify({scope:'production App with mock API',kind:'completed',size,mode,probes,diagnostics,transitions:['A','B','A'],mutations}),contentType:'application/json'});
 });
 test(`Held telemetry cannot replace new register data ${size}/${mode}`,async({page},info)=>{
  await page.setViewportSize({width,height});await page.addInitScript(mode=>localStorage.setItem('beefiscal.ui.preferences.v1',JSON.stringify({language:'en',themeMode:mode})),mode);const mutations=await mockFiscalApi(page,{secondRegister:true});
  let release!:()=>void,reads=0;const gate=new Promise<void>(r=>release=r),errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
  await page.route(`${API}/devices/${DEVICE_ID}/health`,async r=>{reads++;if(reads===1)await gate;await r.fulfill({contentType:'application/json',body:JSON.stringify({id:DEVICE_ID,effective_state:'READY',observed_at:'2026-10-18T00:00:00Z',owner:reads===1?'LATE_RETIRED_A':'FRESH_A'})})});
  await page.route(`${API}/devices/${SECOND_DEVICE_ID}/health`,r=>r.fulfill({contentType:'application/json',body:JSON.stringify({id:SECOND_DEVICE_ID,effective_state:'READY',observed_at:'2026-10-18T00:00:00Z',owner:'CURRENT_B'})}));
  try{await openApp(page);await expect.poll(()=>reads).toBe(1);await page.getByRole('radio',{name:'R02',exact:true}).first().click();await expect(page.getByTestId('device-realtime-health')).toContainText('CURRENT_B');
   const done=page.waitForResponse(r=>r.url()===`${API}/devices/${DEVICE_ID}/health`);release();await(await done).finished();await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));await expect(page.getByTestId('device-realtime-health')).toContainText('CURRENT_B');await expect(page.getByTestId('device-realtime-health')).not.toContainText('LATE_RETIRED_A');
   await page.getByRole('radio',{name:'R01',exact:true}).first().click();await expect(page.getByTestId('device-realtime-health')).toContainText('FRESH_A');expect(mutations).toEqual([]);expect(errors).toEqual([]);await info.attach('owner-proof',{body:JSON.stringify({scope:'production App with mock API',kind:'held',size,mode,reads,transitions:['A held','B confirmed','A response retired','A fresh'],mutations}),contentType:'application/json'});
  }finally{release()}
 });
}
