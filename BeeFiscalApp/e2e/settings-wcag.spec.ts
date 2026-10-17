import { doubleText } from './text-scale';
import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mockFiscalApi, openApp } from './helpers';
import { saveWcagEvidence } from './contrast-evidence';

for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    for (const state of ['populated', 'tax-empty', 'error'] as const) {
      test(`Paper settings WCAG AA CSS200 ${state}/${size}/${mode}`, async ({ page }, testInfo) => {
        const errors: string[] = [];
        page.on('pageerror', error => errors.push(error.message));
        await page.setViewportSize({ width, height });
        await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
        await mockFiscalApi(page, state === 'tax-empty' ? { taxGroupsEmpty: true } : state === 'error' ? { policyStatus: 503 } : {});
        await openApp(page);
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId('tab-Настройки').click();
        await doubleText(page);
        if (state === 'error') {
          const error = page.getByTestId('settings.error');
          await expect(error).toBeVisible();
          const helper = error.getByRole('alert');
          const helperId = await helper.getAttribute('id');
          expect(helperId).toBeTruthy();
          const retry = page.getByTestId('settings.retry');
          await expect(retry).toHaveAttribute('aria-describedby', helperId!);
          expect((await retry.boundingBox())!.height).toBeGreaterThanOrEqual(48);
        }
        else if (state === 'tax-empty') await expect(page.getByTestId('settings.tax-groups')).toContainText('Няма данъчни групи в API.');
        else await expect(page.getByTestId('settings.tax-group.B')).toBeVisible();
        expect(errors).toEqual([]);
        expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
        await saveWcagEvidence(page, testInfo, result, { state: `settings-${state}`, size, width, height, mode });
        expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
        expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
      });
    }
  }
}
