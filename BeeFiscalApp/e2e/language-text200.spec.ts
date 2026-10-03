import { writeFile } from 'node:fs/promises';
import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
const preferencesSource = readFileSync('src/ui/preferences.tsx', 'utf8');
const languageLiteral = preferencesSource.match(/export const languageOptions = \[([\s\S]*?)\] as const;/)?.[1];
if (!languageLiteral) throw new Error('Canonical languageOptions missing');
const languageOptions = Array.from(languageLiteral.matchAll(/\['([^']+)', '([^']+)'\]/g), match => [match[1], match[2]] as const);
if (languageOptions.length !== 25) throw new Error('Expected canonical 25 locales');
import { frameContrast } from './affordance-contrast';
import { doubleText } from './text-scale';
import { mockFiscalApi, openApp } from './helpers';

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper all25 language selection CSS200 ${size}/${theme}`, async ({ page }, testInfo) => {
      test.setTimeout(90_000);
      const errors: string[] = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page);
      await openApp(page);
      await doubleText(page);
      const trigger = page.getByTestId('shell.language.open');
      const evidence = [];
      for (const [index, [code, name]] of languageOptions.entries()) {
        await trigger.focus();
        await page.keyboard.press('Enter');
        const list = page.getByTestId('shell.language.list');
        await expect(page.getByRole('menuitemradio')).toHaveCount(25);
        await page.keyboard.press('Home');
        for (let step = 0; step < index; step++) await page.keyboard.press('ArrowDown');
        const item = page.getByTestId(`shell.language.${code}`);
        await expect(item).toBeFocused();
        await expect(item).toHaveAccessibleName(name);
        const metrics = await item.evaluate(element => {
          const text = element.querySelector<HTMLElement>('[dir="auto"]')!;
          const rect = element.getBoundingClientRect();
          const menu = element.closest('[role="menu"]')!.getBoundingClientRect();
          return { text: text.textContent, size: parseFloat(getComputedStyle(text).fontSize), original: Number(text.dataset.originalFontSize),
            labelWidth: text.clientWidth, labelScrollWidth: text.scrollWidth, labelHeight: text.clientHeight, labelScrollHeight: text.scrollHeight,
            left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, height: rect.height,
            menuTop: menu.top, menuBottom: menu.bottom, menuHeight: menu.height, viewport: innerWidth, focusVisible: element.matches(':focus-visible') };
        });
        expect(metrics.size).toBe(metrics.original * 2);
        expect(metrics.labelScrollWidth).toBeLessThanOrEqual(metrics.labelWidth + 1);
        expect(metrics.labelScrollHeight).toBeLessThanOrEqual(metrics.labelHeight + 1);
        expect(metrics.height).toBeGreaterThanOrEqual(48);
        expect(metrics.menuHeight).toBeLessThanOrEqual(225);
        expect(metrics.top).toBeGreaterThanOrEqual(metrics.menuTop - 1);
        expect(metrics.bottom).toBeLessThanOrEqual(metrics.menuBottom + 1);
        expect(metrics.left).toBeGreaterThanOrEqual(0);
        expect(metrics.right).toBeLessThanOrEqual(metrics.viewport + 1);
        expect(metrics.focusVisible).toBe(true);
        const frame = await frameContrast(item);
        expect(frame.insideRatio).toBeGreaterThanOrEqual(3);
        expect(frame.outsideRatio).toBeGreaterThanOrEqual(3);
        const label = item.locator('[dir="auto"]').first();
        const text = await frameContrast(label, 'color');
        expect(text.insideRatio).toBeGreaterThanOrEqual(4.5);
        evidence.push({ code, name, metrics, frame, text });
        await page.keyboard.press('Enter');
        await expect(trigger).toContainText(code.toUpperCase());
        await expect(trigger).toBeFocused();
        await expect(list).toHaveCount(0);
        await expect.poll(() => page.evaluate(() => JSON.parse(localStorage.getItem('beefiscal.ui.preferences.v1') ?? '{}').language)).toBe(code);
      }
      await writeFile(testInfo.outputPath('language-text200.json'), JSON.stringify({ size, theme, evidence }, null, 2));
      expect(errors).toEqual([]);
      expect(mutations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}
