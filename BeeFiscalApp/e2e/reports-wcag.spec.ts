import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mockFiscalApi, openApp } from './helpers';
import { saveWcagEvidence } from './contrast-evidence';

for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
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
      const stateContent = page.getByTestId(state === 'populated' ? 'reports.item.report-1' : state === 'empty' ? 'reports.empty' : 'reports.error');
      await stateContent.scrollIntoViewIfNeeded();
      await expect(stateContent).toBeVisible();
      const visibleBounds = await stateContent.evaluate(node => {
        const rect = node.getBoundingClientRect();
        const scroll = document.querySelector('[data-testid="operational.content"]')!.getBoundingClientRect();
        return { top: rect.top, bottom: rect.bottom, scrollTop: scroll.top, scrollBottom: scroll.bottom };
      });
      expect(visibleBounds.top).toBeGreaterThanOrEqual(visibleBounds.scrollTop - 1);
      expect(visibleBounds.bottom).toBeLessThanOrEqual(visibleBounds.scrollBottom + 1);
      if (state === 'error') {
        const helper = page.getByTestId('reports.error').getByRole('alert');
        const id = await helper.getAttribute('id');
        expect(id).toBeTruthy();
        const retry = page.getByTestId('reports.retry');
        await retry.scrollIntoViewIfNeeded();
        await expect(retry).toHaveAttribute('aria-describedby', id!);
        expect((await retry.boundingBox())!.height).toBeGreaterThanOrEqual(48);
      }
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
