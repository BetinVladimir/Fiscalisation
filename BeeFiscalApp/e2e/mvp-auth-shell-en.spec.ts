import {expect,test} from '@playwright/test';
import {mockFiscalApi,DEVICE_ID} from './helpers';
import {foundationCopy} from '../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';

// Real signed-out UI and API contracts; no auth/storage injection.
test('EN MVP auth retry, tenant, device navigation and logout hide raw server JSON',async({page})=>{
 await page.setViewportSize({width:360,height:800});
 await mockFiscalApi(page);
 let challenges=0;const bodies:Record<string,unknown>[]=[];
 const tenant={tenant_id:'company-1',display_name:'Test Fiscal Company',roles:['ADMIN']};
 await page.route('http://fiscal-admin.test/public/v1/app-auth/**',async route=>{
  const request=route.request(),path=new URL(request.url()).pathname;
  const body=request.postData()?JSON.parse(request.postData()!):{};
  const json=(value:unknown,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(value)});
  if(path.endsWith('/challenges')){challenges++;bodies.push(body);expect(body.email).toBe('qa@example.test');expect(body.language).toBe('en');expect(body.app_instance_id).toBeTruthy();return challenges===1?json({code:'PRIVATE_BACKEND_FAILURE',details:{sql:'PRIVATE_SQL_TABLE'}},503):json({temporary_token:'temporary-1'});}
  if(path.endsWith('/challenges:verify')){expect(request.headers().authorization).toBe('Bearer temporary-1');return body.code==='123456'?json({tenant_selection_token:'selection-1',tenants:[tenant]}):json({code:'PRIVATE_INVALID_OTP'},400);}
  if(path.endsWith('/tenant-session')){expect(request.headers().authorization).toBe('Bearer selection-1');expect(body.tenant_id).toBe(tenant.tenant_id);return json({access_token:'auth-e2e-token',refresh_token:'refresh-1',expires_in:3600,tenant});}
  if(path.endsWith('/tenants')){expect(request.headers().authorization).toBe('Bearer auth-e2e-token');return json({items:[tenant]});}
  if(path.endsWith('/logout')){expect(body.refresh_token).toBe('refresh-1');return json({});}
  throw Error('Unexpected auth route '+request.method()+' '+path);
 });
 await page.goto('/');
 await page.getByTestId('auth.language.open').click();await page.getByTestId('auth.language.en').click();
 const email=page.getByTestId('admin-login-email');await email.fill('bad');await expect(page.getByTestId('admin-login-start')).toBeDisabled();
 await email.fill('qa@example.test');await page.getByTestId('admin-login-start').click();
 await expect(page.getByTestId('auth.error')).toHaveText(foundationCopy.en.error);await expect(page.getByText(/PRIVATE_BACKEND_FAILURE|PRIVATE_SQL_TABLE/)).toHaveCount(0);await expect(email).toHaveValue('qa@example.test');
 await page.getByTestId('admin-login-start').click();const code=page.getByTestId('admin-login-code');await code.fill('000000');await page.getByTestId('admin-login-verify').click();
 await expect(page.getByTestId('auth.error')).toHaveText(foundationCopy.en.error);await expect(page.getByText('PRIVATE_INVALID_OTP')).toHaveCount(0);await expect(code).toHaveValue('000000');
 await code.fill('123456');await page.getByTestId('admin-login-verify').click();await page.getByRole('button',{name:'Choose company Test Fiscal Company'}).click();
 await expect(page.getByTestId('admin-logout')).toHaveText('Sign out');await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeVisible();
 await page.getByTestId('nav.open').click();await expect(page.getByTestId('tab-Устройства')).toContainText('Devices');await expect(page.getByTestId('tab-Администриране')).toContainText('Administration');await page.getByTestId('tab-Устройства').click();
 await page.getByTestId(`device.item.${DEVICE_ID}`).click();await expect(page.getByTestId('device-readiness')).toBeVisible();
 expect(bodies).toHaveLength(2);await page.getByTestId('admin-logout').click();await expect(email).toBeVisible();await expect(page.getByTestId('admin-logout')).toHaveCount(0);
});
