import { doubleText } from './text-scale';
import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mockFiscalApi, openApp } from './helpers';
import { saveWcagEvidence } from './contrast-evidence';

for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    for (const state of ['populated', 'expanded', 'empty', 'error'] as const) {
      test(`Paper audit WCAG AA ${state}/${size}/${mode}`, async ({ page }, testInfo) => {
        const errors: string[] = [];
        page.on('pageerror', error => errors.push(error.message));
        await page.setViewportSize({ width, height });
        await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
        await mockFiscalApi(page, state === 'empty' ? { auditEmpty: true } : state === 'error' ? { auditStatus: 503 } : {});
        await openApp(page);
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId('tab-Одит').click();
        await expect(page.getByTestId(state === 'empty' ? 'audit.empty' : state === 'error' ? 'audit.error' : 'audit-event-audit-1')).toBeVisible();
        if (state === 'expanded') {
          await page.getByTestId('audit.details.audit-1').click();
          await expect(page.getByTestId('audit.details-content.audit-1')).toBeVisible();
        }
      if (state === 'error') {
        const helper = page.getByTestId('audit.error').getByRole('alert');
        const id = await helper.getAttribute('id');
        expect(id).toBeTruthy();
        const retry = page.getByTestId('audit.retry');
        await retry.scrollIntoViewIfNeeded();
        await expect(retry).toHaveAttribute('aria-describedby', id!);
        expect((await retry.boundingBox())!.height).toBeGreaterThanOrEqual(48);
      }
        expect(errors).toEqual([]);
        expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
        await saveWcagEvidence(page, testInfo, result, { state: `audit-${state}`, size, width, height, mode });
        expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
        expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
      });
    }
  }
}

// Authored only: global forms/test completeness gate defers execution.
for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    test(`Paper audit expanded CSS200 ${size}/${mode}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page);
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Одит').click();
      await doubleText(page);
      const accordion = page.getByTestId('audit.details.audit-1');
      await accordion.scrollIntoViewIfNeeded();
      await accordion.click();
      const details = page.getByTestId('audit.details-content.audit-1');
      await expect(details).toBeVisible();
      await details.scrollIntoViewIfNeeded();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
      const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
      await saveWcagEvidence(page, testInfo, result, { state: 'audit-expanded-css200', size, width, height, mode });
      expect(result.violations).toEqual([]);
      expect(result.incomplete).toEqual([]);
      expect(mutations).toHaveLength(0);
    });
  }
}
