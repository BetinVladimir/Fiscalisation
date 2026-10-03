import { writeFile } from 'node:fs/promises';
import { test, expect } from '@playwright/test';
import { setupMiniposRoutes, setupFiscalRoutes } from './helpers';
import { translations } from '../src/translations.generated';
import { doubleText } from './text-scale';
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper shared onboarding field CSS200 ${size}/${theme}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await setupMiniposRoutes(page); await setupFiscalRoutes(page);
      await page.route('**/auth/verify-code', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ onboarding_required: true, onboarding_token: 'local-e2e-only' }) }));
      await page.goto('/');
      await page.locator('[placeholder="email@example.com"]').fill('qa@example.org');
      await page.getByTestId('operator-login-start').click();
      await page.locator('[placeholder="000000"]').fill('123456');
      await page.getByTestId('operator-login-verify').click();
      await expect(page.getByTestId('onboarding-tax-id')).toBeVisible();
      await doubleText(page);
      const label = page.getByText(translations.bg.taxIdentifier, { exact: true }).first();
      await label.scrollIntoViewIfNeeded();
      const metrics = await label.evaluate(element => ({ text: element.textContent, original: Number((element as HTMLElement).dataset.originalFontSize),
        size: parseFloat(getComputedStyle(element).fontSize), width: element.clientWidth, scrollWidth: element.scrollWidth,
        height: element.clientHeight, scrollHeight: element.scrollHeight }));
      expect(metrics.size).toBe(metrics.original * 2);
      expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.width + 1);
      expect(metrics.scrollHeight).toBeLessThanOrEqual(metrics.height + 1);
      await page.getByTestId('onboarding-tax-id').fill('BG123456789');
      await expect(page.getByTestId('onboarding-tax-id')).toHaveValue('BG123456789');
      await expect(page.getByTestId('onboarding-tax-id')).toHaveAccessibleName(translations.bg.taxIdentifier);
      await expect(page.getByTestId('operator-onboarding-create')).toBeDisabled();
      await writeFile(testInfo.outputPath('field-label.json'), JSON.stringify({ size, theme, metrics }, null, 2));
      await page.screenshot({ path: testInfo.outputPath('field-label.png') });
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}
