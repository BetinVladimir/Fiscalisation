import { test, expect } from '@playwright/test';
import { mockFiscalApi, openApp } from './helpers';
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper language keyboard ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      await openApp(page);
      const trigger = page.getByTestId('shell.language.open');
      await expect(trigger).toHaveAttribute('aria-expanded', 'false');
      await trigger.focus();
      await page.keyboard.press('Enter');
      await expect(trigger).toHaveAttribute('aria-expanded', 'true');
      await expect(trigger).toHaveAttribute('aria-controls', 'shell-language-menu');
      await expect(page.locator('#shell-language-menu')).toHaveCount(1);
      await expect(page.locator('#shell-language-menu')).toHaveAttribute('role', 'menu');
      const selected = page.getByTestId('shell.language.bg');
      await expect(selected).toBeFocused();
      await expect(selected).toHaveAttribute('role', 'menuitemradio');
      await expect(selected).toHaveAttribute('aria-checked', 'true');
      await page.keyboard.press('End');
      await expect(page.getByTestId('shell.language.ru')).toBeFocused();
      const list = page.getByTestId('shell.language.list');
      expect(await list.evaluate(element => element.scrollTop)).toBeGreaterThan(0);
      await page.keyboard.press('Home');
      await expect(selected).toBeFocused();
      await page.keyboard.press('ArrowDown');
      await expect(page.getByTestId('shell.language.hr')).toBeFocused();
      await page.keyboard.press('ArrowUp');
      await expect(selected).toBeFocused();
      await page.keyboard.press('Escape');
      await expect(trigger).toBeFocused();
      await expect(trigger).toHaveAttribute('aria-expanded', 'false');
      await page.keyboard.press('Space');
      await page.keyboard.press('End');
      await page.keyboard.press('Enter');
      await expect(trigger).toContainText('RU');
      await expect(trigger).toBeFocused();
      await expect(list).toHaveCount(0);
      await page.keyboard.press('Space');
      await expect(page.getByTestId('shell.language.ru')).toBeFocused();
      await expect(page.getByTestId('shell.language.ru')).toHaveAttribute('aria-checked', 'true');
      await page.keyboard.press('ArrowDown');
      await expect(selected).toBeFocused();
      await page.keyboard.press('Shift+Tab');
      await expect(trigger).toBeFocused();
      await expect(list).toHaveCount(0);
      await page.keyboard.press('Space');
      await page.keyboard.press('Tab');
      await expect(page.getByTestId('shell.theme.toggle')).toBeFocused();
      await expect(list).toHaveCount(0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}
