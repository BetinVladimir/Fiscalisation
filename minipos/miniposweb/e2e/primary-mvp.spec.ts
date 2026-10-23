import {test,expect} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import {primaryAuthorityFixture} from './primary-authority-fixture';
import {PRODUCTS,ADAPTER_ORIGIN} from './helpers';
import {translations} from '../src/ui/workTranslations.generated';
for(const [width,theme] of [[320,'light'],[768,'light'],[768,'dark'],[1440,'dark']] as const){
 test(`EN primary sale ${width}/${theme}`,async({page},info)=>{
  test.setTimeout(120000);
  const fixture=await primaryAuthorityFixture(page);
  const scans:{state:string;raw:Awaited<ReturnType<AxeBuilder['analyze']>>;paint?:unknown}[]=[],unexpected:string[]=[];
  let release:(()=>void)|undefined,checkoutRequests=0;
  await page.setViewportSize({width,height:900});
  await page.addInitScript(({theme})=>{localStorage.setItem('minipos-language','en');localStorage.setItem('minipos-web-theme',theme)},{theme});
  await page.route('**/*',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname.endsWith('/execution-authorizations:dispatch')){checkoutRequests++;await new Promise<void>(resolve=>{release=resolve});return route.fallback()}
   if(url.origin===new URL(info.project.use.baseURL!).origin||url.pathname.startsWith('/public/v1/minipos/')||url.origin===ADAPTER_ORIGIN)return route.fallback();
   unexpected.push(url.href);return route.abort();
  });
  const scan=async(state:string)=>{
   await page.evaluate(async()=>{await document.fonts.ready});
   // Scan the settled screen, retaining Paper's real press feedback during interaction.
   await expect(page.locator('[data-paper-ripple]')).toHaveCount(0);
   await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
   const raw=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']).analyze();const selectors=raw.incomplete.flatMap(issue=>issue.nodes.flatMap(node=>node.target.filter((v):v is string=>typeof v==='string')));const paint=await page.evaluate(selectors=>selectors.map(selector=>{const e=document.querySelector(selector);if(!e)return{selector,missing:true};const r=e.getBoundingClientRect(),x=Math.min(innerWidth-1,Math.max(0,r.x+r.width/2)),y=Math.min(innerHeight-1,Math.max(0,r.y+r.height/2));return{selector,html:e.outerHTML.slice(0,500),rect:{x:r.x,y:r.y,width:r.width,height:r.height},stack:document.elementsFromPoint(x,y).slice(0,8).map(n=>{const s=getComputedStyle(n);return{tag:n.tagName,testid:n.getAttribute('data-testid'),class:n.className,background:s.backgroundColor,position:s.position,zIndex:s.zIndex,opacity:s.opacity,pointerEvents:s.pointerEvents}})}}),selectors);scans.push({state,raw,paint});
   await info.attach(state,{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
   // Retain every raw result; assert the complete AA matrix after the business flow.
  };
  try{
   await page.goto('/');await expect(page.getByTestId('sale-products')).toBeVisible();await expect.poll(()=>page.evaluate(()=>sessionStorage.getItem('minipos-local-token'))).toBe('local-adapter-token');await scan('catalog');
   await page.getByTestId('sale-product-prod-001').click();await expect(page.getByTestId('sale-total')).toContainText('2.50');await scan('cart');
   const submit=page.getByTestId('sale-checkout');await submit.evaluate(el=>{(el as HTMLElement).click();(el as HTMLElement).click()});await expect.poll(()=>checkoutRequests).toBe(1);await expect(submit).toBeDisabled();await scan('pending');release!();
   await expect(page.getByTestId('sale-outcome-title')).toHaveText('Operation completed');await expect(page.getByTestId('sale-outcome')).toContainText('CLOUD-REF');await scan('outcome');
   await page.getByTestId('app-settings').click();await expect(page.getByTestId('sale-result')).toContainText('CLOUD-REF');await expect(page.getByTestId('sale-result')).not.toContainText('"operation_id":');await expect(page.getByText(translations.en.chooseProduct,{exact:true})).toBeVisible();await scan('result');
   await page.reload();await expect(page.getByTestId('sale-products')).toBeVisible();await expect(submit).toBeDisabled();expect(checkoutRequests).toBe(1);await scan('reload');fixture.verify();expect(unexpected).toEqual([]);expect(scans).toHaveLength(6);expect(scans.flatMap(({state,raw})=>[...raw.violations,...raw.incomplete].map(issue=>({state,id:issue.id,nodes:issue.nodes})))).toEqual([]);
  }finally{release?.();await info.attach('raw-evidence',{body:JSON.stringify({scans,ledger:fixture.ledger,issuances:fixture.issuances,errors:fixture.errors,unexpected,checkoutRequests,scope:'EN fixture-session synthetic cash sale; auth UI/error/reconciliation/native/live/full matrix OPEN'}),contentType:'application/json'})}
 });
}
