import { expect, test } from '@playwright/test';
import { LOCATION_ID, mockFiscalApi, openApp } from './helpers';

// Authored during code phase. Do not run before the global forms/tests gate.
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Reference Paper radios keyboard and filtering ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page, { secondRegister: true });
      await openApp(page);
      const group = page.getByRole('radiogroup', { name: 'Касово място', exact: true }).first();
      const first = group.getByRole('radio', { name: 'R01', exact: true });
      const second = group.getByRole('radio', { name: 'R02', exact: true });
      await expect(first).toBeChecked();
      await first.focus();
      await page.keyboard.press('ArrowDown');
      await expect(second).toBeChecked();
      await expect(second).toBeFocused();
      await expect(first).not.toBeChecked();
      await page.keyboard.press('ArrowDown');
      await expect(first).toBeChecked();
      await expect(first).toBeFocused();
      await page.keyboard.press('End');
      await expect(second).toBeChecked();
      await page.keyboard.press('Home');
      await expect(first).toBeChecked();
      const search = page.getByTestId('reference.search.Касово място').first();
      await search.fill('R02');
      await expect(group.getByRole('radio')).toHaveCount(1);
      await second.focus();
      await page.keyboard.press('Space');
      await expect(second).toBeChecked();
      await search.fill('not-found');
      await expect(group.getByRole('radio')).toHaveCount(0);
      await search.fill('');
      await expect(second).toBeChecked();
      expect(mutations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Admin fields and choices freeze during save ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      let release!: () => void;
      const gate = new Promise<void>(resolve => { release = resolve; });
      let writes = 0;
      await page.route(`**/locations/${LOCATION_ID}`, async route => {
        if (route.request().method() !== 'PATCH') return route.fallback();
        writes++; await gate;
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: LOCATION_ID, code: 'PENDING', name: 'Sofia', version: 2 }) });
      });
      try {
        await openApp(page);
        if (size === 'xs') await page.getByTestId('nav.open').click();
        await page.getByTestId('tab-Администриране').click();
        const field = page.getByTestId('admin-location-code');
        const save = page.getByTestId('admin-location-save');
        const choice = page.getByRole('radio', { name: 'SOF • Sofia', exact: true });
        await field.fill('PENDING');
        await save.click();
        await expect.poll(() => writes).toBe(1);
        await expect(field).not.toBeEditable();
        await expect(choice).toBeDisabled();
        await expect(save).toBeDisabled();
        await choice.evaluate(element => (element as HTMLElement).click());
        await expect(field).toHaveValue('PENDING');
        release();
        await expect(save).toBeEnabled();
        await expect(field).toBeEditable();
        expect(writes).toBe(1);
      } finally { release(); }
    });
  }
}
