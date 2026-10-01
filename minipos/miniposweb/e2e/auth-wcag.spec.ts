import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { setupMiniposRoutes } from './helpers';

test('Paper auth and language menu: WCAG matrix at xs/md/lg in light/dark', async ({ page }) => {
  test.setTimeout(120_000);
  const evidenceDir = join(process.cwd(), 'docs', 'ui-screenshots');
  await mkdir(evidenceDir, { recursive: true });
  const scans: { screen: string; viewport: string; theme: string; violations: unknown[]; incomplete: unknown[] }[] = [];
  await setupMiniposRoutes(page, { verifyCode: { access_token: '', refresh_token: '', expires_in: 0, onboarding_required: true, onboarding_token: 'wcag-onboard' } });
  await page.goto('/');
  for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
    await page.setViewportSize({ width, height });
    for (const mode of ['light', 'dark'] as const) {
      await page.getByTestId('auth-theme-open').click();
      await page.getByTestId(`auth-theme-${mode}`).click();
      await expect(page.locator('html')).toHaveAttribute('data-theme', mode);
      await expect(page.getByTestId('auth-email')).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
      const auth = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
      scans.push({ screen: 'auth-email', viewport: size, theme: mode, violations: auth.violations, incomplete: auth.incomplete });
      await page.screenshot({ path: join(evidenceDir, `auth-email-${size}-${mode}.png`) });

      await page.getByTestId('auth-language-open').click();
      await expect.poll(() => page.evaluate(() => new Set(Array.from(document.querySelectorAll('[data-testid^="auth-language-"]'))
        .map(element => element.getAttribute('data-testid'))
        .filter(value => /^auth-language-[a-z]{2}$/.test(value || ''))).size)).toBe(25);
      await page.getByTestId('auth-language-ru').scrollIntoViewIfNeeded();
      await expect(page.getByTestId('auth-language-ru')).toBeVisible();
      const scroll = await page.getByTestId('auth-language-ru').evaluate(element => {
        let parent = element.parentElement;
        while (parent && parent.scrollHeight <= parent.clientHeight + 1) parent = parent.parentElement;
        return { inner: !!parent && parent !== document.body && parent !== document.documentElement, top: parent?.scrollTop || 0, page: scrollY };
      });
      expect(scroll.inner && scroll.top > 0 && scroll.page === 0, `${size}/${mode}: language menu must scroll inside its own popover`).toBe(true);
      const menu = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
      scans.push({ screen: 'auth-language-menu', viewport: size, theme: mode, violations: menu.violations, incomplete: menu.incomplete });
      await page.screenshot({ path: join(evidenceDir, `auth-language-menu-${size}-${mode}.png`) });
      await page.keyboard.press('Escape');

      await page.getByTestId('auth-theme-open').click();
      const themeMenu = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
      scans.push({ screen: 'auth-theme-menu', viewport: size, theme: mode, violations: themeMenu.violations, incomplete: themeMenu.incomplete });
      await page.screenshot({ path: join(evidenceDir, `auth-theme-menu-${size}-${mode}.png`) });
      await page.keyboard.press('Escape');

      await page.getByTestId('auth-email').fill('operator@example.com');
      await page.getByRole('button', { name: 'Изпрати код' }).click();
      await expect(page.getByTestId('auth-code')).toBeVisible();
      const otp = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
      scans.push({ screen: 'auth-otp', viewport: size, theme: mode, violations: otp.violations, incomplete: otp.incomplete });
      await page.screenshot({ path: join(evidenceDir, `auth-otp-${size}-${mode}.png`) });

      await page.getByTestId('auth-code').fill('123456');
      await page.getByRole('button', { name: 'Продължи' }).click();
      await expect(page.getByTestId('auth-company-name')).toBeVisible();
      await expect(page.getByTestId('auth-theme-open')).toBeInViewport({ ratio: 1 });
      await expect(page.getByTestId('auth-language-open')).toBeInViewport({ ratio: 1 });
      const onboarding = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
      scans.push({ screen: 'auth-onboarding', viewport: size, theme: mode, violations: onboarding.violations, incomplete: onboarding.incomplete });
      await page.screenshot({ path: join(evidenceDir, `auth-onboarding-${size}-${mode}.png`), fullPage: true });
      await page.reload();
    }
  }
  await writeFile(join(process.cwd(), 'docs', 'ui-optimization-auth-axe.json'), `${JSON.stringify({ scans }, null, 2)}\n`);
  expect(scans.flatMap(scan => (scan.violations as { id: string; nodes: unknown[] }[]).map(violation => ({ screen: scan.screen, viewport: scan.viewport, theme: scan.theme, id: violation.id, nodes: violation.nodes.length })))).toEqual([]);
});
