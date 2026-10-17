// Authored only; runtime execution deferred by the global forms/test gate.
import {test,expect} from '@playwright/test';
import {setup} from './helpers';
import {translations} from '../src/ui/workTranslations.generated';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`late token refresh cannot restore logged-out session ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
  await page.clock.install();await setup(page,{minipos:{session:503}});
  let release!:()=>void,requests=0;const held=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/auth/refresh',async route=>{requests++;await held;await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({access_token:'late-access',refresh_token:'late-refresh',expires_in:3600})});});
  await page.goto('/');await expect(page.getByTestId('session-status')).toHaveText(translations.bg.sessionUnavailable);
  await page.clock.fastForward(12*60*1000);await expect.poll(()=>requests).toBe(1);
  await page.clock.fastForward(12*60*1000);expect(requests).toBe(1);
  await page.getByTestId('session-logout').click();await expect(page.getByTestId('auth-email')).toBeVisible();
  const response=page.waitForResponse(response=>response.url().includes('/auth/refresh'));release();await response;
  await expect(page.getByTestId('auth-email')).toBeVisible();
  await expect.poll(()=>page.evaluate(()=>[localStorage.getItem('minipos-access-token'),localStorage.getItem('minipos-refresh-token')])).toEqual([null,null]);
 });
}
