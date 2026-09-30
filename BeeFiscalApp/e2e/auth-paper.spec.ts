import { expect, test } from '@playwright/test';
import { mockFiscalApi } from './helpers';
import { authCopy } from '../src/ui/auth-copy.generated';

for (const locale of ['en', 'bg', 'ru', 'de', 'fr'] as const) {
  test(`Paper auth locale smoke ${locale} on xs`, async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 800 });
    await page.goto('/');
    await page.getByTestId('auth.language.open').click();
    await page.getByTestId(`auth.language.${locale}`).click();
    await expect(page.getByText(authCopy[locale].title)).toBeVisible();
    const login = await page.getByTestId('admin-login').boundingBox();
    expect(login).not.toBeNull();
    expect(login!.x).toBeGreaterThanOrEqual(0);
    expect(login!.x + login!.width).toBeLessThanOrEqual(361);
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
  });
}

test('Paper language menu scrolls through 25 locales and theme/language persist', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 720 });
  await page.emulateMedia({ colorScheme: 'light', reducedMotion: 'reduce' });
  const challengeBodies: Record<string, unknown>[] = [];
  await page.route('http://fiscal-admin.test/public/v1/app-auth/challenges', route => {
    challengeBodies.push(JSON.parse(route.request().postData() ?? '{}') as Record<string, unknown>);
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ temporary_token: 'temporary-1' }) });
  });
  await page.goto('/');
  await page.getByTestId('auth.language.open').click();
  await expect(page.getByRole('menuitem')).toHaveCount(25);
  await page.getByTestId('auth.language.ru').click();
  await expect(page.getByText('BeeFiscal • вход администратора')).toBeVisible();
  await page.getByTestId('auth.language.open').click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('menuitem')).toHaveCount(0);
  await page.getByTestId('auth.theme.toggle').click();
  await page.getByTestId('auth.theme.toggle').click();
  await expect(page.getByTestId('auth.root')).toHaveCSS('background-color', 'rgb(13, 10, 18)');
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
  await page.reload();
  await expect(page.getByText('BeeFiscal • вход администратора')).toBeVisible();
  await expect(page.getByTestId('auth.root')).toHaveCSS('background-color', 'rgb(13, 10, 18)');
  await page.getByTestId('admin-login-email').fill('qa@example.org');
  await page.getByTestId('admin-login-start').click();
  await expect(page.getByTestId('admin-login-code')).toBeVisible();
  expect(challengeBodies[0]?.language).toBe('ru');
});

test('production Paper auth retains input after errors and selects a company', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 });
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
  await mockFiscalApi(page);
  const calls: Array<{ path: string; body: Record<string, unknown> }> = [];
  let challengeAttempts = 0;
  await page.route('http://fiscal-admin.test/public/v1/app-auth/**', async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const body = request.postData() ? JSON.parse(request.postData()!) as Record<string, unknown> : {};
    calls.push({ path, body });
    const json = (value: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(value) });
    if (path.endsWith('/challenges')) {
      challengeAttempts += 1;
      return challengeAttempts === 1 ? json({ code: 'TEMPORARY_FAILURE' }, 503) : json({ temporary_token: 'temporary-1' });
    }
    if (path.endsWith('/challenges:verify')) {
      return body.code === '123456' ? json({ tenant_selection_token: 'selection-1', tenants: [{ tenant_id: 'company-1', display_name: 'Test Fiscal Company', roles: ['ADMIN'] }] }) : json({ code: 'INVALID_CODE' }, 400);
    }
    if (path.endsWith('/tenant-session')) return json({ access_token: 'auth-e2e-token', refresh_token: 'refresh-1', expires_in: 3600, tenant: { tenant_id: 'company-1', display_name: 'Test Fiscal Company', roles: ['ADMIN'] } });
    if (path.endsWith('/tenants')) return json({ items: [{ tenant_id: 'company-1', display_name: 'Test Fiscal Company', roles: ['ADMIN'] }] });
    return json({ code: 'NOT_FOUND' }, 404);
  });

  await page.goto('/');
  const login = page.getByTestId('admin-login');
  await expect(login).toBeVisible();
  const background = await page.locator('body').evaluate(element => getComputedStyle(element).backgroundColor);
  expect(background).toBeTruthy();
  const email = page.getByTestId('admin-login-email');
  await email.fill('qa@example.org');
  await page.getByTestId('admin-login-start').click();
  await expect(page.getByText(/TEMPORARY_FAILURE/)).toBeVisible();
  await expect(email).toHaveValue('qa@example.org');
  await page.getByTestId('admin-login-start').click();
  const code = page.getByTestId('admin-login-code');
  await expect(code).toBeVisible();
  await code.fill('000000');
  await page.getByTestId('admin-login-verify').click();
  await expect(page.getByText(/INVALID_CODE/)).toBeVisible();
  await expect(code).toHaveValue('000000');
  await code.fill('123456');
  await page.getByTestId('admin-login-verify').click();
  await expect(page.getByRole('button', { name: 'Изберете компания Test Fiscal Company' })).toBeVisible();
  await page.getByRole('button', { name: 'Изберете компания Test Fiscal Company' }).click();
  await expect(page.getByTestId('status-fiscal-device')).toBeVisible();
  expect(calls.filter(call => call.path.endsWith('/challenges')).map(call => call.body.email)).toEqual(['qa@example.org', 'qa@example.org']);
  expect(calls.find(call => call.path.endsWith('/tenant-session'))?.body.tenant_id).toBe('company-1');
});
