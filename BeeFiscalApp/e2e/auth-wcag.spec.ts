import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { writeFile } from 'node:fs/promises';

for (const [size, width, height] of [
  ['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000],
] as const) for (const mode of ['light', 'dark'] as const) {
  test(`production Paper email WCAG AA ${size}/${mode}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height });
    await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
    await page.goto('/');
    const login = page.getByTestId('admin-login');
    await expect(login).toBeVisible();
    await expect(page.getByTestId('admin-login-email')).toBeVisible();
    const box = await login.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
    if (size === 'md') expect(box!.width).toBeLessThanOrEqual(width * 0.55);
    if (size === 'lg') expect(box!.width).toBeLessThanOrEqual(width * 0.4);
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
    await page.screenshot({ path: testInfo.outputPath('screen.png'), fullPage: true });
    await writeFile(testInfo.outputPath('axe.json'), JSON.stringify({ size, width, height, mode, violations: result.violations, incomplete: result.incomplete }, null, 2));
    expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
    expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
  });

  test(`production Paper language list WCAG AA ${size}/${mode}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height });
    await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
    await page.goto('/');
    await expect(page.getByTestId('admin-login-email')).toBeVisible();
    await page.getByTestId('auth.language.open').click();
    await expect(page.getByRole('menuitem')).toHaveCount(25);
    const box = await page.getByTestId('auth.language.list').boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeLessThanOrEqual(225);
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
    await page.screenshot({ path: testInfo.outputPath('screen.png'), fullPage: true });
    await writeFile(testInfo.outputPath('axe.json'), JSON.stringify({ size, width, height, mode, state: 'language-open', violations: result.violations, incomplete: result.incomplete }, null, 2));
    expect(result.violations.map(item => item.id), 'Axe violations').toEqual([]);
    expect(result.incomplete.map(item => item.id), 'Axe incomplete needs manual resolution').toEqual([]);
  });
}
