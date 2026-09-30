import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { writeFile } from 'node:fs/promises';
import { mockFiscalApi } from './helpers';

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

for (const [size, width, height] of [
  ['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000],
] as const) for (const mode of ['light', 'dark'] as const) {
  test(`production Paper email WCAG AA ${size}/${mode}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height });
    await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
    await page.goto('/');
    const login = page.getByTestId('admin-login');
    await expect(login).toBeVisible();
    await expect(page.getByTestId('admin-login-email')).toBeVisible();
    const box = await login.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
    if (size === 'md') expect(box!.width).toBeLessThanOrEqual(width * 0.55);
    if (size === 'lg') expect(box!.width).toBeLessThanOrEqual(width * 0.4);
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
    await page.screenshot({ path: testInfo.outputPath('screen.png'), fullPage: true });
    await writeFile(testInfo.outputPath('axe.json'), JSON.stringify({ size, width, height, mode, violations: result.violations, incomplete: result.incomplete }, null, 2));
    expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
    expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
  });
}
