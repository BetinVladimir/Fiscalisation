import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mockFiscalApi, openApp } from './helpers';
import { saveWcagEvidence } from './contrast-evidence';

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    for (const state of ['populated', 'empty', 'error'] as const) {
    test(`Paper reports WCAG AA ${state}/${size}/${mode}`, async ({ page }, testInfo) => {
      const errors: string[] = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page, state === 'empty' ? { reportsEmpty: true } : state === 'error' ? { reportsStatus: 503 } : {});
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Отчети').click();
      await expect(page.getByTestId('reports.screen')).toBeVisible();
      await expect(page.getByTestId(state === 'populated' ? 'reports.item.report-1' : state === 'empty' ? 'reports.empty' : 'reports.error')).toBeVisible();
      expect(errors).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
      const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
      await saveWcagEvidence(page, testInfo, result, { state: `reports-${state}`, size, width, height, mode });
      expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
      expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
    });
    }
  }
}
