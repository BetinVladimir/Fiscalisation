import { expect, test } from '@playwright/test';
import { mockFiscalApi, openApp } from './helpers';

// Deferred: code-phase checks must all be authored before runtime execution.
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    for (const kind of ['operations', 'reports'] as const) {
      test(`Invalid ${kind} identity excludes commands and retries ${size}/${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height });
        await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
        const mutations = await mockFiscalApi(page);
        let attempts = 0;
        await page.route(`**/public/v1/${kind}?*`, async route => {
          if (route.request().method() !== 'GET') return route.fallback();
          attempts++;
          const item = kind === 'operations'
            ? { id: 'operation-unknown-1', type: 'FISCAL_SALE', state: 'UNKNOWN', allowed_actions: ['RECONCILE'] }
            : { id: 'report-1', type: 'X', state: 'COMPLETED' };
          await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: attempts === 1 ? [item, item] : [item], page: { has_more: false } }) });
        });
        await openApp(page);
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId(kind === 'operations' ? 'tab-Операции' : 'tab-Отчети').click();
        await expect(page.getByTestId(`${kind}.error`)).toBeVisible();
        if (kind === 'operations') await expect(page.getByTestId('operation-reconcile-operation-unknown-1')).toHaveCount(0);
        else await expect(page.getByTestId('reports.create.X')).toBeDisabled();
        await page.getByTestId(kind === 'operations' ? 'beefiscal.state.retry' : 'reports.retry').click();
        await expect(page.getByTestId(`${kind}.error`)).toHaveCount(0);
        if (kind === 'operations') await expect(page.getByTestId('operation-reconcile-operation-unknown-1')).toBeEnabled();
        else await expect(page.getByTestId('reports.create.X')).toBeEnabled();
        expect(attempts).toBe(2);
        expect(mutations).toEqual([]);
      });
    }
  }
}
