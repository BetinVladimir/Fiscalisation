import { doubleText } from './text-scale';
import { expect, test } from '@playwright/test';
import { foundationCopy } from '../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import { mockFiscalApi } from './helpers';
import { authCopy } from '../src/ui/auth-copy.generated';

for (const locale of ['en', 'bg', 'ru', 'de', 'fr'] as const) {
  test(`Paper auth locale smoke ${locale} on xs`, async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 800 });
    await page.addInitScript(() => {
      window.open = ((url?: string | URL) => {
        window.localStorage.setItem('auth.lastLegalUrl', String(url));
        return null;
      }) as typeof window.open;
    });
    await page.goto('/');
    await page.getByTestId('auth.language.open').click();
    await page.getByTestId(`auth.language.${locale}`).click();
    await expect(page.getByTestId('admin-login').getByRole('heading',{name:authCopy[locale].title,exact:true})).toBeVisible();
    const login = await page.getByTestId('admin-login').boundingBox();
    expect(login).not.toBeNull();
    expect(login!.x).toBeGreaterThanOrEqual(0);
    expect(login!.x + login!.width).toBeLessThanOrEqual(361);
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    const privacy = page.getByRole('link', { name: authCopy[locale].privacyPolicy });
    expect(await privacy.locator('..').evaluate(node => parseFloat(getComputedStyle(node).borderTopWidth))).toBeGreaterThanOrEqual(1);
    expect((await privacy.boundingBox())!.height).toBeGreaterThanOrEqual(48);
    await privacy.click();
    expect(await page.evaluate(() => localStorage.getItem('auth.lastLegalUrl'))).toBe(`https://legal.beeloy.org/${locale}/privacy`);
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
  await expect(page.getByRole('menuitemradio')).toHaveCount(25);
  await page.getByTestId('auth.language.ru').click();
  await expect(page.getByTestId('admin-login').getByRole('heading',{name:'BeeFiscal • вход администратора',exact:true})).toBeVisible();
  await page.getByTestId('auth.language.open').click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('menuitemradio')).toHaveCount(0);
  await page.getByTestId('auth.theme.toggle').click();
  await page.getByTestId('auth.theme.toggle').click();
  await expect(page.getByTestId('auth.surface')).toHaveCSS('background-color', 'rgb(13, 10, 18)');
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
  await page.reload();
  await expect(page.getByTestId('admin-login').getByRole('heading',{name:'BeeFiscal • вход администратора',exact:true})).toBeVisible();
  await expect(page.getByTestId('auth.surface')).toHaveCSS('background-color', 'rgb(13, 10, 18)');
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
  await expect(page.getByTestId('auth.error')).toHaveText(foundationCopy.bg.error);
  await expect(page.getByText(/TEMPORARY_FAILURE/)).toHaveCount(0);
  await expect(email).toHaveValue('qa@example.org');
  await page.getByTestId('admin-login-start').click();
  const code = page.getByTestId('admin-login-code');
  await expect(code).toBeVisible();
  await code.fill('000000');
  await page.getByTestId('admin-login-verify').click();
  await expect(page.getByTestId('auth.error')).toHaveText(foundationCopy.bg.error);
  await expect(page.getByText(/INVALID_CODE/)).toHaveCount(0);
  await expect(code).toHaveValue('000000');
  await code.fill('123456');
  await page.getByTestId('admin-login-verify').click();
  await expect(page.getByRole('button', { name: 'Изберете компания Test Fiscal Company' })).toBeVisible();
  await page.getByRole('button', { name: 'Изберете компания Test Fiscal Company' }).click();
  await expect(page.getByTestId('status-fiscal-device')).toBeVisible();
  await expect(page.getByTestId('admin-logout')).toBeVisible();
  let release!:()=>void;const held=new Promise<void>(resolve=>{release=resolve});let printerCalls=0;
  await page.route('http://fiscal-admin.test/public/v1/**',async route=>{const request=route.request(),url=new URL(request.url());if(request.method()!=='POST'||!url.pathname.endsWith('/tests/printer'))return route.fallback();printerCalls++;await held;return route.fallback();});
  const authRequests=calls.length;
  try{await page.getByTestId('device-printer-test').click();await expect.poll(()=>printerCalls).toBe(1);await expect(page.getByTestId('admin-logout')).toBeDisabled();await expect(page.getByTestId('nav.open')).toBeDisabled();expect(calls).toHaveLength(authRequests);}finally{release();}
  await expect(page.getByTestId('admin-logout')).toBeEnabled();expect(printerCalls).toBe(1);
  await page.getByTestId('admin-logout').click();
  await expect(page.getByTestId('auth.root')).toBeVisible();
  expect(calls.filter(call => call.path.endsWith('/challenges')).map(call => call.body.email)).toEqual(['qa@example.org', 'qa@example.org']);
  expect(calls.find(call => call.path.endsWith('/tenant-session'))?.body.tenant_id).toBe('company-1');
});


test('Privacy Paper action keeps a visible frame on xs md lg in both system themes', async ({ page }) => {
  let lightBackground='';
  for(const theme of ['light','dark'] as const)for(const width of [360,960,1440]){
    await page.emulateMedia({colorScheme:theme});await page.setViewportSize({width,height:width===360?800:width===1440?1000:900});await page.goto('/');
    const action=page.getByTestId('auth.privacy');await expect(action).toBeVisible();await expect(action).toHaveRole('link');
    const frame=await action.locator('..').evaluate(node=>{const s=getComputedStyle(node);return {width:parseFloat(s.borderTopWidth),style:s.borderTopStyle,color:s.borderTopColor};});
    expect(frame.width).toBeGreaterThanOrEqual(1);expect(frame.style).toBe('solid');expect(frame.color).not.toBe('rgba(0, 0, 0, 0)');expect((await action.boundingBox())!.height).toBeGreaterThanOrEqual(48);
    const background=await page.getByTestId('auth.surface').evaluate(node=>getComputedStyle(node).backgroundColor);if(theme==='light')lightBackground=background;else expect(background).not.toBe(lightBackground);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
  }
});


for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper auth field labels CSS200 ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const calls: unknown[] = [];
      await page.route('http://fiscal-admin.test/public/v1/app-auth/challenges', route => {
        calls.push(JSON.parse(route.request().postData() ?? '{}'));
        return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ temporary_token: 'temporary-e2e' }) });
      });
      await page.goto('/');
      await expect(page.getByTestId('admin-login-email')).toBeVisible();
      await doubleText(page);
      for (const stage of ['email', 'code'] as const) {
        const field = page.getByTestId(`admin-login-${stage}`);
        const label = page.getByTestId(`admin-login-${stage}.label`);
        await expect(label).toHaveText(stage === 'email' ? authCopy.bg.email : authCopy.bg.code);
        await label.scrollIntoViewIfNeeded();
        const metrics = await label.evaluate(element => {
          const text = element.querySelector<HTMLElement>('[dir="auto"]') ?? element as HTMLElement;
          return { original: Number(text.dataset.originalFontSize), size: parseFloat(getComputedStyle(text).fontSize), width: text.clientWidth, scroll: text.scrollWidth };
        });
        expect(metrics.size).toBe(metrics.original * 2);
        expect(metrics.scroll).toBeLessThanOrEqual(metrics.width + 1);
        await expect(field).toHaveAccessibleName(stage === 'email' ? authCopy.bg.email : authCopy.bg.code);
        const labelId = await label.getAttribute('id');
        expect(labelId).toBeTruthy();
        await expect(field).toHaveAttribute('aria-labelledby', labelId!);
        if (stage === 'code') await expect(field).toHaveAttribute('autocomplete', 'one-time-code');
        await field.fill(stage === 'email' ? 'qa@example.org' : '123456');
        if (stage === 'email') await page.getByTestId('admin-login-start').click();
        else { await expect(field).toHaveValue('123456'); await expect(page.getByTestId('admin-login-verify')).toBeEnabled(); }
      }
      expect(calls).toHaveLength(1);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}
