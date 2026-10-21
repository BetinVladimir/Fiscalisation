import { expect, test } from '@playwright/test';
import { DEVICE_ID, mockFiscalApi, openApp } from './helpers';

// Authored during code phase; runtime execution remains globally deferred.
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Pending diagnostics locks commands and navigation without replay ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page);
      let release!: () => void;
      const gate = new Promise<void>(resolve => { release = resolve; });
      let reads = 0;
      await page.route(`**/devices/${DEVICE_ID}/diagnostics`, async route => {
        reads++; await gate;
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ driver: 'late-diagnostic', secrets_redacted: true }) });
      });
      const navigate = async (tab: string) => {
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId(`tab-${tab}`).click();
      };
      try {
        await openApp(page);
        const diagnostic = page.getByTestId('device-diagnostics');
        await diagnostic.click(); await expect.poll(() => reads).toBe(1);
        await expect(diagnostic).toBeDisabled();
        await expect(page.getByTestId('device-printer-test')).toBeDisabled();
        await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeDisabled();
        await expect(page.getByTestId('ble-client-public-key')).not.toBeEditable();
        await page.getByTestId('device-printer-test').evaluate(node => (node as HTMLElement).click());
        await expect(page.getByTestId('nav.open')).toBeDisabled();
        for (const tab of await page.getByTestId(/^tab-/).all()) await expect(tab).toBeDisabled();
        const response = page.waitForResponse(r => r.url().endsWith(`/devices/${DEVICE_ID}/diagnostics`));
        release(); await (await response).finished();
        await expect(page.getByTestId('diagnostics-result')).toContainText('late-diagnostic');
        await expect(page.getByTestId('nav.open')).toBeEnabled();
        await navigate('Операции'); await expect(page.getByTestId('operation-unknown')).toBeVisible();
        await expect(page.getByTestId('status-transport')).toContainText('Операции:');
        await navigate('Устройства');
        await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeVisible();
        await expect(page.getByTestId('diagnostics-result')).toHaveCount(0);
        expect(mutations).toEqual([]); expect(reads).toBe(1);
      } finally { release(); }
    });

    test(`Changing activation code invalidates prior preview ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page);
      await openApp(page);
      const code = page.getByTestId('smart-device-activation-code');
      await code.fill('BC1234');
      await page.getByTestId('smart-device-activation-lookup').click();
      await expect(page.getByTestId('smart-device-activation-preview')).toContainText('BC-001');
      await expect(page.getByTestId('bluecash-activate')).toBeEnabled();
      await code.fill('OTHER');
      await expect(page.getByTestId('smart-device-activation-preview')).toHaveCount(0);
      await expect(page.getByTestId('bluecash-activate')).toBeDisabled();
      await page.getByTestId('bluecash-activate').evaluate(node => (node as HTMLElement).click());
      expect(mutations).toEqual([]);
    });
  }
}
