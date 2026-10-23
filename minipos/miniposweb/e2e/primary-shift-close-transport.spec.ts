import {test,expect} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import {primaryAuthorityFixture} from './primary-authority-fixture';
import {SHIFT,TEST_JWT} from './helpers';
for(const[width,theme]of[[320,'light'],[1440,'dark']]as const)test(`EN lost shift close response ${width}/${theme}`,async({page},info)=>{
 const base=await primaryAuthorityFixture(page),calls:any[]=[],scans:any[]=[];let closeRequests=0,closed=false;
 await page.setViewportSize({width,height:900});await page.addInitScript(({theme})=>{localStorage.setItem('minipos-language','en');localStorage.setItem('minipos-web-theme',theme)},{theme});
 await page.route('**/public/v1/minipos/**',async route=>{
  const req=route.request(),u=new URL(req.url()),suffix=u.pathname.split('/public/v1/minipos')[1];if(suffix!=='/shifts'&&!suffix.startsWith('/shifts/'))return route.fallback();
  expect(req.headers().authorization).toBe('Bearer '+TEST_JWT);expect(req.headers()['x-app-instance-id']).toBeTruthy();calls.push({method:req.method(),path:suffix,query:u.search,key:req.headers()['idempotency-key']});
  if(req.method()==='GET'){expect(suffix).toBe('/shifts');expect(u.search).toBe('?employee_id=emp-001&state=');return route.fulfill({json:{items:closed?[]:[{...SHIFT,state:closeRequests?'CLOSING':'OPEN',allowed_actions:closeRequests?['READ']:['SALE','CLOSE']}],page:{has_more:false,next_cursor:null}}})}
  expect(req.method()).toBe('POST');expect(suffix).toBe('/shifts/'+SHIFT.id+'/close');expect(req.postDataJSON()).toEqual({});expect(req.headers()['idempotency-key']).toBeTruthy();expect(++closeRequests).toBe(1);return route.abort('connectionreset');
 });
 const scan=async(state:string)=>{await page.evaluate(async()=>{await document.fonts.ready});await expect(page.locator('[data-paper-ripple]')).toHaveCount(0);const raw=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']).analyze();scans.push({state,raw});await info.attach(state,{body:await page.screenshot({fullPage:true}),contentType:'image/png'});expect([...raw.violations,...raw.incomplete]).toEqual([])};
 try{
  await page.goto('/');await expect(page.getByTestId('shift-close-open')).toBeEnabled();await page.getByTestId('shift-close-open').click();await page.getByTestId('shift-close-confirm').click();
  await expect(page.getByTestId('shift-close-refresh')).toBeVisible();await expect(page.getByTestId('shift-close-open')).toBeDisabled();await expect(page.getByTestId('sale-product-prod-001')).toBeDisabled();await scan('lost-response');
  await page.getByTestId('shift-close-refresh').click();await expect(page.getByTestId('shift-close-open')).toBeDisabled();await expect(page.getByTestId('shift-close-refresh')).toBeEnabled();await scan('read-only-pending');
  await page.reload();await expect(page.getByTestId('shift-close-open')).toBeDisabled();await expect(page.getByTestId('sale-product-prod-001')).toBeDisabled();await scan('restored-pending');
  closed=true;await page.getByTestId('shift-close-refresh').click();await expect(page.getByTestId('shift-open')).toBeVisible();await scan('server-confirmed-closed');
  expect(closeRequests).toBe(1);expect(calls.filter(r=>r.method==='POST')).toHaveLength(1);base.verifyNoSale();
 }finally{await info.attach('raw-evidence',{body:JSON.stringify({scans,calls,baseLedger:base.ledger,errors:base.errors,scope:'Synthetic lost close response, read-only recovery; no fiscal hardware'}),contentType:'application/json'})}
});
