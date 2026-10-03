import { writeFile } from 'node:fs/promises';
import { test, expect, type Locator } from '@playwright/test';
import { doubleText } from './text-scale';
import { mockFiscalApi, openApp } from './helpers';

async function labelFits(label: Locator) {
  await label.scrollIntoViewIfNeeded();
  const metrics = await label.evaluate(element => {
    const text = element.querySelector<HTMLElement>('[dir="auto"]') ?? element as HTMLElement;
    const rect = text.getBoundingClientRect();
    return { original: Number(text.dataset.originalFontSize), size: parseFloat(getComputedStyle(text).fontSize),
      width: text.clientWidth, scroll: text.scrollWidth, height: text.clientHeight, scrollHeight: text.scrollHeight,
      left: rect.left, right: rect.right, viewport: innerWidth };
  });
  expect(metrics.size).toBe(metrics.original * 2);
  expect(metrics.scroll).toBeLessThanOrEqual(metrics.width + 1);
  expect(metrics.scrollHeight).toBeLessThanOrEqual(metrics.height + 1);
  expect(metrics.left).toBeGreaterThanOrEqual(0);
  expect(metrics.right).toBeLessThanOrEqual(metrics.viewport + 1);
  return metrics;
}
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper administrative field labels CSS200 ${size}/${theme}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page);
      await openApp(page);
      await doubleText(page);
      const deviceMetrics = await labelFits(page.getByTestId('devices.search.label'));
      await page.getByTestId('devices.search').fill('Datecs');
      await expect(page.getByTestId('devices.search')).toHaveValue('Datecs');
      const search = page.getByTestId('reference.search.Касово място').first();
      const searchLabel = page.getByTestId('reference.search.Касово място.label').first();
      await expect(searchLabel).toHaveText('Търсене: Касово място');
      const searchMetrics = await labelFits(searchLabel);
      await search.fill('missing');
      await expect(page.getByText('Няма съвпадения.', { exact: true })).toBeVisible();
      await search.fill('');
      const longLabel = page.getByTestId('ble-client-public-key.label');
      await expect(longLabel).toHaveText('Подготвен X25519 client public key');
      const longMetrics = await labelFits(longLabel);
      await page.getByTestId('ble-client-public-key').fill('e2e-local-only');
      await expect(page.getByTestId('ble-client-public-key')).toHaveValue('e2e-local-only');
      await page.screenshot({ path: testInfo.outputPath('long-label.png') });
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Администриране').click();
      const label = page.getByTestId('admin-location-code.label');
      await expect(label).toHaveText('Код на точката');
      const fieldMetrics = await labelFits(label);
      const field = page.getByTestId('admin-location-code');
      await expect(field).toHaveAccessibleName('Код на точката');
      await field.fill('E2E');
      await expect(field).toHaveValue('E2E');
      const jsonMetrics = await labelFits(page.getByTestId('admin-device-transport-parameters.label'));
      await page.getByTestId('admin-device-transport-parameters').fill('{invalid');
      await expect(page.getByText('Въведете валиден JSON обект.', { exact: true })).toBeVisible();
      await expect(page.getByTestId('admin-device-save')).toBeDisabled();
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Одит').click();
      const auditMetrics = [];
      for (const name of ['actor', 'action', 'objectType', 'objectId', 'unp', 'from', 'to']) {
        auditMetrics.push(await labelFits(page.getByTestId(`audit.filter.${name}.label`)));
        await page.getByTestId(`audit.filter.${name}`).fill('e2e-filter');
        await expect(page.getByTestId(`audit.filter.${name}`)).toHaveValue('e2e-filter');
      }
      await writeFile(testInfo.outputPath('label-metrics.json'), JSON.stringify({ size, theme, searchMetrics, fieldMetrics, longMetrics, deviceMetrics, jsonMetrics, auditMetrics }, null, 2));
      await testInfo.attach('label-metrics', { body: JSON.stringify({ size, theme, searchMetrics, fieldMetrics, longMetrics, deviceMetrics, jsonMetrics, auditMetrics }), contentType: 'application/json' });
      expect(mutations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}
