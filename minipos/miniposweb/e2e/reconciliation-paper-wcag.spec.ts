import {test,expect} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import {readFile} from 'node:fs/promises';
import {reconciliationCatalogues} from '../src/ui/reconciliation-copy.generated';
const editorial=JSON.parse(await readFile(new URL('../src/ui/reconciliation-copy.editorial.json',import.meta.url),'utf8'));
const reconciliationCopyFor=(language:string):Record<string,string>=>({...reconciliationCatalogues[language],...editorial[language]});
const fixturePath=process.env.RECONCILIATION_SIGNED_UI_FIXTURE;
for(const language of ['en','bg','ru'])for(const width of [320,960,1440])for(const theme of ['light','dark'])for(const scale of [1,2])for(const outcome of ['review','error'])test(`reconciliation Paper ${language} ${width} ${theme} zoom=${scale} ${outcome}`,async({page},info)=>{
 test.setTimeout(60000);if(!fixturePath)throw Error('Real Go-signed reconciliation fixture required');
 const fixture=JSON.parse(await readFile(fixturePath,'utf8')),copy=reconciliationCopyFor(language),errors:string[]=[],unexpected:string[]=[];let fixtureReads=0;
 page.on('pageerror',error=>errors.push(error.message));await page.setViewportSize({width,height:900});
 await page.addInitScript(mode=>localStorage.setItem('minipos-web-theme',mode),theme);
 // Fixed instant only for this component presentation test; expiry logic is tested separately.
 await page.clock.setFixedTime(new Date((fixture.grant.nbf+1)*1000));
 await page.route('**/*',async route=>{const url=new URL(route.request().url());if(url.origin!==new URL(info.project.use.baseURL!).origin){unexpected.push(url.origin+url.pathname);return route.abort()}
  if(url.pathname==='/__reconciliation_fixture'){expect(route.request().method()).toBe('GET');fixtureReads++;expect(fixtureReads).toBe(1);return route.fulfill({json:fixture})}
  if(url.pathname.includes('/public/')||url.pathname.includes('/beeloy/local/')){unexpected.push(url.pathname);return route.abort()}
  return route.continue();
 });
 const scan=async(state:string)=>{await expect(page.getByTestId('payment-reconciliation')).toBeVisible();await page.evaluate(value=>{document.documentElement.style.zoom=String(value)},scale);await page.emulateMedia({reducedMotion:'reduce'});await page.evaluate(()=>document.fonts.ready);await page.waitForTimeout(200);
  const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']).analyze();await info.attach(state+'-raw-aa',{body:JSON.stringify(result),contentType:'application/json'});await info.attach(state+'-screenshot',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
  expect(result.violations).toEqual([]);expect(result.incomplete).toEqual([]);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 };
 try{
  await page.goto(`/e2e/reconciliation-paper-harness.html?language=${language}&outcome=${outcome}`);await expect(page.getByTestId('payment-reconciliation')).toBeVisible();
  const group=page.getByRole('radiogroup',{name:copy.observedState,exact:true}),radios=group.getByRole('radio');await expect(radios).toHaveCount(5);await expect(radios.nth(0)).toHaveAttribute('aria-checked','true');await expect(group.locator('[role="radio"][tabindex="0"]')).toHaveCount(1);
  await radios.nth(0).focus();await page.keyboard.press('ArrowDown');await expect(radios.nth(1)).toHaveAttribute('aria-checked','true');await expect(radios.nth(1)).toBeFocused();await page.keyboard.press('End');await expect(radios.nth(4)).toHaveAttribute('aria-checked','true');await page.keyboard.press('Home');await expect(radios.nth(0)).toHaveAttribute('aria-checked','true');await expect(group.locator('[role="radio"][tabindex="0"]')).toHaveCount(1);await scan('form');
  for(const [key,label]of [['operation_id',copy.originalOperation],['payment_id',copy.payment_id],['evidence_reference',copy.evidence_reference],['evidence_sha256',copy.evidence_sha256],['terminal_reference',copy.terminal_reference],['rrn',copy.rrn],['auth_code',copy.auth_code]] as const)await page.getByLabel(label,{exact:true}).fill(fixture.decision[key]);
  await page.getByRole('button',{name:copy.request,exact:true}).click();
  if(outcome==='error'){await expect(page.getByText(copy.error,{exact:true})).toBeVisible({timeout:15000});await expect(page.getByLabel(copy.originalOperation,{exact:true})).toHaveValue(fixture.decision.operation_id);await expect(page.locator('body')).not.toContainText('PRIVATE_BROWSER_BANK_SECRET');}
  else {await expect(page.getByTestId('payment-reconciliation.comparison')).toBeVisible({timeout:15000});await expect(page.getByRole('button',{name:copy.confirm,exact:true})).toBeEnabled();expect(await page.getByTestId('payment-reconciliation.grant').textContent()).toContain(fixture.grant.evidence_sha256);}
  // Optional empty reversal references are absent in a new decision, as in the production JSON request.
  const expectedDecision=Object.fromEntries(Object.entries(fixture.decision).filter(([key,value])=>!key.startsWith('reversal_')||value!==''));
  await scan(outcome);expect(await page.evaluate(()=>(window as any).__reconciliationProof.authorizations)).toEqual([expectedDecision]);expect(errors).toEqual([]);expect(unexpected).toEqual([]);expect(fixtureReads).toBe(1);
 }finally{await info.attach('component-boundary-proof',{body:JSON.stringify({language,width,theme,scale,outcome,fixtureReads,errors,unexpected,scope:'Genuine component only; no full App login/transport/dispatch/native acceptance'}),contentType:'application/json'});}
});
