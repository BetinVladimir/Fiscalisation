import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mockFiscalApi, openApp } from './helpers';
import { saveWcagEvidence } from './contrast-evidence';

for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    for (const state of ['populated', 'empty', 'error'] as const) {
    test(`Paper operations WCAG AA ${state}/${size}/${mode}`, async ({ page }, testInfo) => {
      const errors: string[] = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page, state === 'empty' ? { operationsEmpty: true } : state === 'error' ? { operationsStatus: 503 } : {});
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Операции').click();
      await expect(page.getByTestId('operations.screen')).toBeVisible();
      await expect(page.getByTestId(state === 'populated' ? 'operation-unknown' : `operations.${state}`)).toBeVisible();
      expect(errors).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
      const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
      await saveWcagEvidence(page, testInfo, result, { state: `operations-${state}`, size, width, height, mode });
      expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
      expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
    });
    }
  }
}
