import { writeFile } from 'node:fs/promises';
import { test, expect, type Locator } from '@playwright/test';
import { frameContrast } from './affordance-contrast';
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
  const contrast = await frameContrast(label, 'color');
  expect(contrast.insideRatio, 'Label text contrast').toBeGreaterThanOrEqual(4.5);
  return { ...metrics, contrast };
}
for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
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
      const visibleLabelId=await label.getAttribute('id');expect(visibleLabelId).toBeTruthy();await expect(field).toHaveAttribute('aria-labelledby',visibleLabelId!);
      await field.fill('E2E');
      await expect(field).toHaveValue('E2E');
      const jsonMetrics = await labelFits(page.getByTestId('admin-device-transport-parameters.label'));
      await page.getByTestId('admin-device-transport-parameters').fill('{invalid');
      const jsonError = page.getByText('Въведете валиден JSON обект.', { exact: true });
      await expect(jsonError).toBeVisible();
      const errorContrast = await frameContrast(jsonError, 'color');
      expect(errorContrast.insideRatio, 'JSON error text contrast').toBeGreaterThanOrEqual(4.5);
      await expect(page.getByTestId('admin-device-save')).toBeDisabled();
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Одит').click();
      const auditMetrics = [];
      for (const name of ['actor', 'action', 'objectType', 'objectId', 'unp', 'from', 'to']) {
        auditMetrics.push(await labelFits(page.getByTestId(`audit.filter.${name}.label`)));
        await page.getByTestId(`audit.filter.${name}`).fill('e2e-filter');
        await expect(page.getByTestId(`audit.filter.${name}`)).toHaveValue('e2e-filter');
      }
      await writeFile(testInfo.outputPath('label-metrics.json'), JSON.stringify({ size, theme, searchMetrics, fieldMetrics, longMetrics, deviceMetrics, jsonMetrics, auditMetrics, errorContrast }, null, 2));
      await testInfo.attach('label-metrics', { body: JSON.stringify({ size, theme, searchMetrics, fieldMetrics, longMetrics, deviceMetrics, jsonMetrics, auditMetrics, errorContrast }), contentType: 'application/json' });
      expect(mutations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}


test('text contrast negative control detects invisible Paper label', async ({ page }) => {
  await mockFiscalApi(page);
  await openApp(page);
  const label = page.getByTestId('devices.search.label');
  const baseline = await frameContrast(label, 'color');
  expect(baseline.insideRatio).toBeGreaterThanOrEqual(4.5);
  const original = await label.getAttribute('style');
  await label.evaluate((element, background) => {
    (element as HTMLElement).style.color = `rgb(${background.slice(0, 3).join(',')})`;
  }, baseline.inside);
  try {
    const invisible = await frameContrast(label, 'color');
    expect(invisible.insideRatio).toBe(1);
  } finally {
    await label.evaluate((element, style) => { if (style === null) element.removeAttribute('style'); else element.setAttribute('style', style); }, original);
  }
  expect((await frameContrast(label, 'color')).insideRatio).toBeGreaterThanOrEqual(4.5);
});
