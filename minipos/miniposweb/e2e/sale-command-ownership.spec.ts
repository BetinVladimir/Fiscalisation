// Authored only; runtime deferred until the global forms/test gate.
import {test,expect} from '@playwright/test';
import {setup,waitForPOS} from './helpers';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`sale command synchronous admission ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setup(page);
  let release!:()=>void,writes=0;const held=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/public/v1/minipos/orders',async route=>{if(route.request().method()!=='POST')return route.fallback();writes++;await held;await route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({code:'ORDER_REJECTED'})});});
  await page.goto('/');await waitForPOS(page);await page.getByRole('button',{name:'Кафе',exact:true}).click();
  const checkout=page.getByTestId('sale-checkout');await checkout.evaluate(node=>{node.dispatchEvent(new MouseEvent('click',{bubbles:true}));node.dispatchEvent(new MouseEvent('click',{bubbles:true}));});
  await expect.poll(()=>writes).toBe(1);await expect(checkout).toBeDisabled();await expect(page.getByTestId('sale-card-amount')).not.toBeEditable();
  await expect(page.getByRole('button',{name:'Кафе',exact:true})).toBeDisabled();
  release();await expect(page.getByTestId('sale-message')).toBeVisible();await expect(checkout).toBeEnabled();expect(writes).toBe(1);
  await expect(page.getByTestId('sale-total')).toContainText('2.50');
 });
}
