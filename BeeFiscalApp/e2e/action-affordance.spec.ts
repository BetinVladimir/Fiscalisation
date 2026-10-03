import { test, expect, type Locator } from '@playwright/test';
import { mockFiscalApi, openApp } from './helpers';

async function expectFrame(action: Locator) {
  await action.scrollIntoViewIfNeeded();
  const frame = await action.evaluate(element => {
    const style = getComputedStyle(element);
    const rect = element.getBoundingClientRect();
    return { width: parseFloat(style.borderTopWidth), style: style.borderTopStyle,
      color: style.borderTopColor, height: rect.height, left: rect.left, right: rect.right,
      viewport: innerWidth };
  });
  expect(frame.width).toBeGreaterThanOrEqual(1);
  expect(frame.style).toBe('solid');
  expect(frame.color).not.toBe('rgba(0, 0, 0, 0)');
  expect(frame.height).toBeGreaterThanOrEqual(48);
  expect(frame.left).toBeGreaterThanOrEqual(0);
  expect(frame.right).toBeLessThanOrEqual(frame.viewport + 1);
}

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper navigation and reference affordance ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page, { secondRegister: true });
      await openApp(page);
      const reference = page.getByRole('button', { name: 'R02', exact: true }).first();
      await expectFrame(reference);
      await reference.focus();
      await page.keyboard.press('Enter');
      await expect(page.getByRole('button', { name: 'Избрано: R02', exact: true }).first()).toBeVisible();
      if (size === 'xs') await page.getByTestId('nav.open').click();
      const navigation = page.getByTestId('tab-Администриране').getByRole('button');
      await expectFrame(navigation);
      await navigation.focus();
      await page.keyboard.press('Space');
      await expect(page.getByTestId('admin-editors')).toBeVisible();
      const choice = page.getByRole('toolbar', { name: 'Съществуващи записи' }).getByRole('button', { name: 'Избрано: SOF • Sofia', exact: true });
      await expectFrame(choice);
      await page.getByTestId('admin-location-code').fill('CHANGED');
      await choice.focus();
      await page.keyboard.press('Enter');
      await expect(page.getByTestId('admin-location-code')).toHaveValue('SOF');
      await expect(page.getByRole('button', { name: 'Избрано: SOF • Sofia', exact: true })).toBeVisible();
      expect(mutations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}
