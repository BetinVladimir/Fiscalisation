import { expect, test } from '@playwright/test';
import { LOCATION_ID, mockFiscalApi, openApp } from './helpers';

// Deferred until the global forms/Paper/authored-tests gate.
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Pending admin save locks navigation and preserves its draft without replay ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      let release!: () => void;
      const gate = new Promise<void>(resolve => { release = resolve; });
      let writes = 0;
      await page.route(`**/locations/${LOCATION_ID}`, async route => {
        if (route.request().method() !== 'PATCH') return route.fallback();
        writes++; await gate;
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: LOCATION_ID, code: 'DRAFT', name: 'Sofia', version: 2 }) });
      });
      const navigate = async (tab: string) => {
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId(`tab-${tab}`).click();
      };
      try {
        await openApp(page); await navigate('Администриране');
        await expect(page.getByTestId('admin-location-save')).toBeEnabled();
        await page.getByTestId('admin-location-code').fill('DRAFT');
        await page.getByTestId('admin-location-save').click();
        await expect.poll(() => writes).toBe(1);
        await expect(page.getByTestId('admin-operator-save')).toBeDisabled();
        await page.getByTestId('admin-location-save').evaluate(node => (node as HTMLElement).click());
        await expect(page.getByTestId('nav.open')).toBeDisabled();
        for (const tab of await page.getByTestId(/^tab-/).all()) await expect(tab).toBeDisabled();
        await expect(page.getByTestId('admin-location-code')).not.toBeEditable();
        await expect(page.getByTestId('admin-location-code')).toHaveValue('DRAFT');
        const completed = page.waitForResponse(r => r.request().method() === 'PATCH' && r.url().endsWith(`/locations/${LOCATION_ID}`));
        release(); await (await completed).finished();
        await expect(page.getByTestId('nav.open')).toBeEnabled();
        await expect(page.getByTestId('admin-location-code')).toHaveValue('DRAFT');
        await navigate('Операции'); await expect(page.getByTestId('operation-unknown')).toBeVisible();
        await expect(page.getByTestId('status-transport')).toContainText('Операции:');
        await navigate('Администриране');
        await expect(page.getByTestId('admin-location-save')).toBeEnabled();
        await expect(page.getByTestId('admin-location-code')).toHaveValue('DRAFT');
        // Saving this record must retain its selected identity after navigation.
        await expect(page.getByRole('radio', { name: 'SOF • Sofia', exact: true })).toBeChecked();
        expect(writes).toBe(1);
      } finally { release(); }
    });
  }
}

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Disabling pending binding reads real register version ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      const register = '00000000-0000-4000-8000-000000000001';
      const binding = { binding_id: '00000000-0000-4000-8000-000000000099', register_id: register, status: 'PENDING', generation: 2, version: 2 };
      let body: Record<string, unknown> | null = null;
      await page.route(`**/registers/${register}/composite-bindings`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [binding] }) }));
      await page.route(`**/registers/${register}/composite-bindings/disable`, route => {
        body = JSON.parse(route.request().postData() || '{}');
        return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ ...binding, status: 'REVOKED', version: 3 }) });
      });
      await page.route(`**/registers/${register}`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: register, code: 'R01', version: 1 }) }));
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Администриране').click();
      await expect(page.getByTestId('composite-binding-state')).toContainText('PENDING');
      await page.getByTestId('composite-binding-disable').click();
      await expect(page.getByTestId('composite-binding-state')).toContainText('REVOKED');
      await expect(page.getByTestId('composite-binding-disable')).toBeDisabled();
      await expect(page.getByTestId('composite-register-version')).toHaveValue('1');
      expect(body).toEqual({ binding_id: binding.binding_id, expected_version: 2 });
    });
  }
}
