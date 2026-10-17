import { expect, test } from '@playwright/test';
import { mockFiscalApi, openApp, REGISTER_ID } from './helpers';

// Deferred until the global Paper/forms/authored-tests gate is complete.
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Late operations read cannot clear reports loading ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      let releaseOperations!: () => void, releaseReports!: () => void;
      const operationsGate = new Promise<void>(resolve => { releaseOperations = resolve; });
      const reportsGate = new Promise<void>(resolve => { releaseReports = resolve; });
      let operationsStarted = false, reportsStarted = false;
      await page.route('**/public/v1/operations?*', async route => {
        operationsStarted = true; await operationsGate; await route.fallback();
      });
      await page.route('**/public/v1/reports?*', async route => {
        reportsStarted = true; await reportsGate; await route.fallback();
      });
      const navigate = async (tab: string) => {
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId(`tab-${tab}`).click();
      };
      try {
        await openApp(page); await navigate('Операции');
        await expect.poll(() => operationsStarted).toBe(true);
        await navigate('Отчети'); await expect.poll(() => reportsStarted).toBe(true);
        const stale = page.waitForResponse(response => new URL(response.url()).pathname.endsWith('/operations'));
        releaseOperations(); await (await stale).finished();
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve(null)))));
        await expect(page.getByTestId('reports.loading')).toBeVisible();
        await expect(page.getByTestId('reports.error')).toHaveCount(0);
        releaseReports();
        await expect(page.getByTestId('reports.item.report-1')).toBeVisible();
        await expect(page.getByTestId('reports.loading')).toHaveCount(0);
        await expect(page.getByTestId('status-transport')).toContainText('Отчети:');
      } finally { releaseOperations(); releaseReports(); }
    });

    test(`Late report command cannot overwrite operations context ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      let release!: () => void;
      const gate = new Promise<void>(resolve => { release = resolve; });
      let writes = 0;
      await page.route(`**/registers/${REGISTER_ID}/reports`, async route => {
        if (route.request().method() !== 'POST') return route.fallback();
        writes++; await gate;
        await route.fulfill({ contentType: 'application/json', status: 202, body: JSON.stringify({ id: 'late-report', state: 'COMPLETED', fiscal_reference: 'LATE' }) });
      });
      const navigate = async (tab: string) => {
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId(`tab-${tab}`).click();
      };
      try {
        await openApp(page); await navigate('Отчети');
        await expect(page.getByTestId('reports.create.X')).toBeEnabled();
        await page.getByTestId('reports.create.X').click(); await expect.poll(() => writes).toBe(1);
        await navigate('Операции');
        await expect(page.getByTestId('operation-unknown')).toBeVisible();
        const stale = page.waitForResponse(response => response.request().method() === 'POST' && response.url().includes(`/registers/${REGISTER_ID}/reports`));
        release(); await (await stale).finished();
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve(null)))));
        await expect(page.getByTestId('operation-unknown')).toBeVisible();
        await expect(page.getByTestId('reports.screen')).toHaveCount(0);
        await expect(page.getByTestId('status-transport')).toContainText('Операции:');
        await expect(page.getByTestId('status-transport')).not.toContainText('LATE');
        expect(writes).toBe(1);
      } finally { release(); }
    });
  }
}
