// Authored only; runtime deferred until global forms/test readiness.
import {test,expect} from '@playwright/test';
import {setup,waitForPOS} from './helpers';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper management pending/error ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setup(page);
  let release!:()=>void;const held=new Promise<void>(resolve=>{release=resolve;});let writes=0;
  await page.route('**/public/v1/minipos/products**',async route=>{if(route.request().method()!=='POST')return route.fallback();writes++;await held;await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:'PRIVATE_SERVER_DETAIL'})});});
  await page.goto('/');await waitForPOS(page);await page.getByTestId('app-products').click();await page.getByTestId('management-add').click();
  const name=page.getByTestId('management-product-name');await name.fill('Retained fixture');await page.getByTestId('management-product-price').fill('2.50');
  const labelId=await page.getByTestId('management-product-name.label').getAttribute('id');expect(labelId).toBeTruthy();await expect(name).toHaveAttribute('aria-labelledby',labelId!);
  await page.getByTestId('management-save').click();await expect.poll(()=>writes).toBe(1);await expect(name).not.toBeEditable();await expect(page.getByTestId('management-cancel')).toBeDisabled();release();
  const error=page.getByTestId('management-error');await expect(error).toBeVisible();await expect(error).toHaveRole('alert');await expect(error).not.toContainText('PRIVATE_SERVER_DETAIL');
  const id=await error.getAttribute('id');expect(id).toBeTruthy();await expect(name).toHaveAttribute('aria-describedby',id!);await expect(name).toHaveValue('Retained fixture');await expect(name).toBeEditable();expect(writes).toBe(1);
 });
}

for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper product price validation and Enter ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setup(page);
  let writes=0;await page.route('**/public/v1/minipos/products**',route=>{
   if(route.request().method()!=='POST')return route.fallback();writes++;
   const body=route.request().postDataJSON();expect(body.name).toBe('Normalized');expect(body.price.amount).toBe('2.50');
   return route.fulfill({status:503,body:''});
  });
  await page.goto('/');await waitForPOS(page);await page.getByTestId('app-products').click();await page.getByTestId('management-add').click();
  const name=page.getByTestId('management-product-name'),price=page.getByTestId('management-product-price'),save=page.getByTestId('management-save');await name.fill(' Normalized ');
  for(const invalid of ['','0','-1','1e2','2.555','Infinity','0.001']){await price.fill(invalid);await expect(save).toBeDisabled();await price.press('Enter');expect(writes).toBe(0);}
  await price.fill('2,50');await expect(save).toBeEnabled();await price.press('Enter');await expect(page.getByTestId('management-error')).toBeVisible();expect(writes).toBe(1);await expect(name).toHaveValue(' Normalized ');await expect(price).toHaveValue('2,50');
 });
}
