import { test, expect } from '@playwright/test';
import { setupMiniposRoutes, setupFiscalRoutes } from './helpers';
import { legalTranslations } from '../src/legalTranslations.generated';

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper onboarding legal actions ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await setupMiniposRoutes(page);
      await setupFiscalRoutes(page);
      await page.route('**/auth/verify-code', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ onboarding_required: true, onboarding_token: 'local-e2e-only' }) }));
      await page.addInitScript(() => { window.open = ((url?: string | URL) => { const state = window as unknown as { legalUrl: string; legalUrls?: string[] }; state.legalUrl = String(url); (state.legalUrls ??= []).push(String(url)); return null; }) as typeof window.open; });
      await page.route('https://legal.beeloy.org/**', route => route.abort());
      await page.goto('/');
      await page.locator('[placeholder="email@example.com"]').fill('qa@example.org');
      await page.getByTestId('operator-login-start').click();
      await page.locator('[placeholder="000000"]').fill('123456');
      await page.getByTestId('operator-login-verify').click();
      await expect(page.getByTestId('onboarding-company-name')).toBeVisible();
      for (const [kind, name] of [['terms', legalTranslations.bg.terms], ['privacy', legalTranslations.bg.privacy]] as const) {
        const action = page.getByRole('button', { name, exact: true }).or(page.getByRole('link', { name, exact: true }));
        await action.scrollIntoViewIfNeeded();
        const frame = await action.locator('..').evaluate(element => { const s = getComputedStyle(element); return { border: parseFloat(s.borderTopWidth), style: s.borderTopStyle, color: s.borderTopColor }; });
        expect(frame.border).toBeGreaterThanOrEqual(1);
        expect(frame.style).toBe('solid');
        expect(frame.color).not.toBe('rgba(0, 0, 0, 0)');
        expect((await action.boundingBox())!.height).toBeGreaterThanOrEqual(48);
        await expect(action).toHaveAttribute('role', 'link');
        await expect(action).toHaveAttribute('href', `https://legal.beeloy.org/bg/${kind}`);
        await page.evaluate(() => { (window as unknown as { legalUrls: string[] }).legalUrls = []; });
        await action.focus();
        await page.keyboard.press('Enter');
        await expect.poll(() => page.evaluate(() => (window as unknown as { legalUrl: string }).legalUrl)).toBe(`https://legal.beeloy.org/bg/${kind}`);
        expect(await page.evaluate(() => (window as unknown as { legalUrls: string[] }).legalUrls)).toEqual([`https://legal.beeloy.org/bg/${kind}`]);
        await action.click();
        await expect.poll(() => page.evaluate(() => (window as unknown as { legalUrls: string[] }).legalUrls.length)).toBe(2);
      }
      await expect(page.getByTestId('operator-onboarding-create')).toBeDisabled();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}
