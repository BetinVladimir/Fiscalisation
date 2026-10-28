import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mockFiscalApi, openApp } from './helpers';
import { saveWcagEvidence } from './contrast-evidence';

for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    for (const state of ['devices-populated', 'devices-disconnected', 'devices-unavailable', 'devices-empty', 'devices-no-matches', 'devices-error', 'admin-top', 'admin-device-form', 'admin-invalid-json'] as const) {
      test(`Paper devices/admin WCAG AA ${state}/${size}/${mode}`, async ({ page }, testInfo) => {
        const errors: string[] = [];
        page.on('pageerror', error => errors.push(error.message));
        await page.setViewportSize({ width, height });
        await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
        await mockFiscalApi(page, state === 'devices-disconnected' ? { devicesDisconnected: true }
          : state === 'devices-unavailable' ? { deviceReady: false }
          : state === 'devices-empty' ? { devicesEmpty: true }
          : state === 'devices-error' ? { devicesStatus: 503 } : {});
        if (state === 'devices-error') {
          await page.goto('/');
          const error = page.getByTestId('devices.error');
          await expect(error).toBeVisible();
          const id = await error.getByRole('alert').getAttribute('id');
          expect(id).toBeTruthy();
          await expect(page.getByTestId('devices.retry')).toHaveAttribute('aria-describedby', id!);
        } else {
          await openApp(page);
          if (state.startsWith('admin-')) {
            if (size === 'xs') await page.getByTestId('nav.open').click();
            await page.getByTestId('tab-Администриране').click();
            await expect(page.getByTestId('admin-editors')).toBeVisible();
            if (state === 'admin-device-form' || state === 'admin-invalid-json') {
              await page.getByTestId('admin-device-save').scrollIntoViewIfNeeded();
              await expect(page.getByTestId('admin-device-save')).toBeVisible();
              if (state === 'admin-invalid-json') {
                await page.getByTestId('admin-device-transport-parameters').fill('{invalid');
                const helper = page.getByRole('alert').filter({ hasText: 'Въведете валиден JSON обект.' });
                await expect(helper).toBeVisible();
                const helperId = await helper.getAttribute('id');
                expect(helperId).toBeTruthy();
                await expect(page.getByTestId('admin-device-transport-parameters')).toHaveAttribute('aria-describedby', helperId!);
                await expect(page.getByTestId('admin-device-transport-parameters')).toHaveAttribute('aria-invalid', 'true');
                await expect(page.getByTestId('admin-device-save')).toBeDisabled();
              }
            }
          } else if (state === 'devices-empty') await expect(page.getByTestId('devices.empty')).toBeVisible();
          else if (state === 'devices-no-matches') {
            await page.getByTestId('devices.search').fill('unknown printer');
            await expect(page.getByTestId('devices.no-matches')).toBeVisible();
          }
          else {
            await expect(page.getByTestId('device.item.00000000-0000-4000-8000-000000000003')).toBeVisible();
            await expect(page.getByTestId('device-readiness')).toHaveText(
              state === 'devices-populated' ? 'Готовност: Готово' : 'Готовност: Не е готово',
            );
          }
        }
        expect(errors).toEqual([]);
        expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
        await saveWcagEvidence(page, testInfo, result, { state, size, width, height, mode });
        expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
        expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
      });
    }
  }
}
