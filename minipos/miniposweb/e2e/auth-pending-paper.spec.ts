// Authored only; runtime deferred until all apps' forms/tests are complete.
import {test,expect} from '@playwright/test';
import {setupMiniposRoutes} from './helpers';
const genericError = 'Възникна проблем. Моля, опитайте отново.';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper auth delayed failure ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setupMiniposRoutes(page);
  let release!:()=>void;const held=new Promise<void>(resolve=>{release=resolve;});let requests=0;
  await page.route('**/auth/request-code',async route=>{requests++;expect(route.request().postDataJSON().email).toBe('retained@example.org');await held;await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:'PRIVATE_AUTH_DETAIL'})});});
  await page.goto('/');const email=page.getByTestId('auth-email');await email.fill('retained@example.org');
  const send=page.getByRole('button',{name:'Изпрати код',exact:true});await send.click();await expect.poll(()=>requests).toBe(1);await expect(send).toBeDisabled();await expect(email).not.toBeEditable();release();
  const error=page.getByTestId('auth-error');await expect(error).toHaveText(genericError);await expect(error).toHaveRole('alert');await expect(error).not.toContainText('PRIVATE_AUTH_DETAIL');const id=await error.getAttribute('id');expect(id).toBeTruthy();await expect(email).toHaveAttribute('aria-describedby',id!);await expect(email).toHaveValue('retained@example.org');await expect(email).toBeEditable();await expect(send).toBeEnabled();expect(requests).toBe(1);await expect(page.getByTestId('auth-code')).toHaveCount(0);
 });
}

for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 for(const malformed of ['session','onboarding','discriminator'] as const)test(`Paper auth malformed ${malformed} ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setupMiniposRoutes(page);
  await page.route('**/auth/verify-code',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(malformed==='session'?{access_token:'',refresh_token:'invalid',expires_in:3600,onboarding_required:false}:malformed==='onboarding'?{onboarding_required:true,onboarding_token:''}:{access_token:'access',refresh_token:'refresh',expires_in:3600,onboarding_required:'false'})}));
  await page.goto('/');await page.getByTestId('auth-email').fill('test@example.org');await page.getByRole('button',{name:'Изпрати код',exact:true}).click();
  await page.getByTestId('auth-code').fill('123456');await page.getByRole('button',{name:'Продължи',exact:true}).click();
  await expect(page.getByTestId('auth-error')).toHaveText(genericError);
  await expect(page.getByTestId('auth-code')).toBeEditable();await expect(page.getByTestId('auth-company-name')).toHaveCount(0);
  expect(await page.evaluate(()=>[localStorage.getItem('minipos-access-token'),localStorage.getItem('minipos-refresh-token')])).toEqual([null,null]);
 });
}

for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper auth email validation and normalized request ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setupMiniposRoutes(page);
  let requests=0;await page.route('**/auth/request-code',route=>{requests++;expect(route.request().postDataJSON().email).toBe('operator@example.org');return route.fulfill({status:204,body:''});});
  await page.goto('/');const email=page.getByTestId('auth-email'),send=page.getByRole('button',{name:'Изпрати код',exact:true});
  await email.fill('invalid');await expect(send).toBeDisabled();expect(requests).toBe(0);
  await email.fill(' operator@example.org ');await expect(send).toBeEnabled();await send.click();
  await expect(page.getByTestId('auth-code')).toBeVisible();expect(requests).toBe(1);
 });
}

for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper auth keyboard submission ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setupMiniposRoutes(page);
  let requested=0,verified=0;
  await page.route('**/auth/request-code',route=>{requested++;return route.fulfill({status:204,body:''});});
  await page.route('**/auth/verify-code',route=>{verified++;return route.fulfill({status:503,body:''});});
  await page.goto('/');const email=page.getByTestId('auth-email');await email.fill('invalid');await email.press('Enter');expect(requested).toBe(0);
  await email.fill('operator@example.org');await email.press('Enter');await expect(page.getByTestId('auth-code')).toBeVisible();expect(requested).toBe(1);
  const code=page.getByTestId('auth-code');await code.fill('12');await code.press('Enter');expect(verified).toBe(0);
  await code.fill('123456');await code.press('Enter');await expect(page.getByTestId('auth-error')).toHaveText(genericError);expect(verified).toBe(1);await expect(code).toHaveValue('123456');await expect(code).toBeEditable();
  const form=await page.locator('.auth-card').boundingBox();expect(form).not.toBeNull();expect(form!.x).toBeGreaterThanOrEqual(0);expect(form!.x+form!.width).toBeLessThanOrEqual(width+1);
  if(size!=='xs')expect(form!.width).toBeLessThanOrEqual(width*.55);
 });
}
