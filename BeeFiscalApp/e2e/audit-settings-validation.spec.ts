import { expect, test } from '@playwright/test';
import { mockFiscalApi, openApp } from './helpers';

// Authored only. Runtime execution remains behind the global Paper + tests gate.
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Audit date bounds exclude invalid requests ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page);
      const auditRequests: string[] = [];
      page.on('request', request => { if (new URL(request.url()).pathname.endsWith('/audit-events')) auditRequests.push(request.url()); });
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Одит').click();
      await expect(page.getByTestId('audit-event-audit-1')).toBeVisible();
      const from = page.getByTestId('audit.filter.from'), to = page.getByTestId('audit.filter.to');
      const apply = page.getByTestId('apply-audit-filters');
      await from.fill('2026-02-30T00:00:00Z');
      await expect(from).toHaveAttribute('aria-invalid', 'true');
      await expect(apply).toBeDisabled();
      await apply.evaluate(node => (node as HTMLElement).click());
      expect(auditRequests).toHaveLength(1);
      await from.fill('2026-10-13T00:00:00Z'); await to.fill('2026-10-12T00:00:00Z');
      await expect(apply).toBeDisabled();
      await from.fill('2026-10-12T00:00:00+03:00');
      await expect(apply).toBeEnabled(); await apply.click();
      await expect.poll(() => auditRequests.length).toBe(2);
      await expect(page.getByTestId('audit-event-audit-1')).toBeVisible();
      expect(new URL(auditRequests[1]).searchParams.get('from')).toBe('2026-10-12T00:00:00+03:00');
      const details = page.getByTestId('audit.details.audit-1');
      await details.click();
      const detailsId = await page.getByTestId('audit.details-content.audit-1').getAttribute('id');
      expect(detailsId).toBeTruthy(); await expect(details).toHaveAttribute('aria-controls', detailsId!);
      expect(mutations).toEqual([]);
    });

    test(`Malformed tax records show recoverable error ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page);
      let reads = 0;
      await page.route('**/public/v1/tax-groups', route => {
        reads++;
        return route.fulfill({ contentType: 'application/json', body: JSON.stringify(reads === 1 ? [null] : [{ code: 'B', rate: '20.00', policy_version: '2026-08-07' }]) });
      });
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Настройки').click();
      await expect(page.getByTestId('settings.error')).toBeVisible();
      await expect(page.getByTestId('settings.policy')).toHaveCount(0);
      await page.getByTestId('settings.retry').click();
      await expect(page.getByTestId('settings.tax-group.B')).toBeVisible();
      await expect(page.getByTestId('settings.error')).toHaveCount(0);
      expect(reads).toBe(2); expect(mutations).toEqual([]);
    });
  }
}
