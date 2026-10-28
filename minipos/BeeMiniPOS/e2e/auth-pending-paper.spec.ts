// Authored only; global forms/test completeness gate defers execution.
import {expect,test} from '@playwright/test';
import {setupMiniposRoutes,setupFiscalRoutes} from './helpers';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const){
 for(const theme of ['light','dark'] as const){
  test(`Paper auth pending and draft ${size}/${theme}`,async({page})=>{
   await page.setViewportSize({width,height});
   await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
   await setupMiniposRoutes(page);await setupFiscalRoutes(page);
   let release!:()=>void;const pending=new Promise<void>(resolve=>{release=resolve;});
   let requests=0;
   await page.route('**/auth/request-code',async route=>{
    requests++;expect(route.request().postDataJSON().email).toBe('pending@example.org');
    await pending;await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:'UNAVAILABLE'})});
   });
   await page.goto('/');
   const email=page.getByTestId('operator-login-email');
   await email.fill('pending@example.org');
   const start=page.getByTestId('operator-login-start');await start.click();
   await expect(start).toBeDisabled();await expect(email).not.toBeEditable();
   await expect.poll(()=>requests).toBe(1);
   release();
   const error=page.getByTestId('operator-login-error');await expect(error).toBeVisible();
   await expect(error).toHaveRole('alert');const id=await error.getAttribute('id');expect(id).toBeTruthy();
   await expect(email).toHaveAttribute('aria-describedby',id!);
   await expect(email).toHaveValue('pending@example.org');await expect(email).toBeEditable();
   await expect(start).toBeEnabled();expect(requests).toBe(1);
  });
 }
}
