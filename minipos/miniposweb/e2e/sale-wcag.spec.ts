import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { setup, waitForPOS } from './helpers';
import { collectContrastCandidates } from './contrast-candidates';

test('Paper sale, cart, payment and diagnostics: xs/md/lg × light/dark', async ({ page }) => {
  test.setTimeout(180_000);
  const evidenceDir = join(process.cwd(), 'docs', 'ui-screenshots');
  await mkdir(evidenceDir, { recursive: true });
  const scans: { screen: string; viewport: string; theme: string; violations: unknown[]; incomplete: unknown[]; contrastCandidates: unknown[] }[] = [];
  await setup(page);
  for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
    await page.setViewportSize({ width, height });
    for (const mode of ['light', 'dark'] as const) {
      await page.addInitScript(theme => localStorage.setItem('minipos-web-theme', theme), mode);
      await page.goto('/');
      await waitForPOS(page);
      await expect(page.locator('html')).toHaveAttribute('data-theme', mode);
      await expect(page.getByTestId('sale-products')).toBeVisible();
      await expect(page.getByTestId('sale-payment')).toBeVisible();
      await scan('sale-empty');
      await page.getByTestId('sale-product-prod-001').click();
      await expect(page.getByTestId('sale-total')).toContainText('2.50');
      await scan('sale-cart');
      await page.getByTestId('sale-card-amount').fill('1.00');
      await expect(page.getByTestId('sale-checkout')).toBeEnabled();
      await scan('sale-split-payment');
      await page.getByRole('button', { name: 'Настройки' }).click();
      await expect(page.getByTestId('sale-diagnostics')).toBeVisible();
      await scan('sale-diagnostics');

      async function scan(screen: string) {
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${screen}/${size}/${mode}: horizontal overflow`).toBe(true);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
        const screenshot = await page.screenshot({ path: join(evidenceDir, `${screen}-${size}-${mode}.png`), fullPage: true });
        scans.push({ screen, viewport: size, theme: mode, violations: result.violations, incomplete: result.incomplete, contrastCandidates: await collectContrastCandidates(page, result, screenshot) });
      }
    }
  }
  await writeFile(join(process.cwd(), 'docs', 'ui-optimization-sale-axe.json'), `${JSON.stringify({ scans }, null, 2)}\n`);
  expect(scans.flatMap(scan => (scan.violations as { id: string; nodes: unknown[] }[]).map(violation => ({
    screen: scan.screen, viewport: scan.viewport, theme: scan.theme, id: violation.id, nodes: violation.nodes.length,
  })))).toEqual([]);
});

test('Paper shift opening: xs/md/lg × light/dark', async ({ page }) => {
  test.setTimeout(120_000);
  const evidenceDir = join(process.cwd(), 'docs', 'ui-screenshots');
  await mkdir(evidenceDir, { recursive: true });
  const scans: { screen: string; viewport: string; theme: string; violations: unknown[]; incomplete: unknown[]; contrastCandidates: unknown[] }[] = [];
  await setup(page, { minipos: { shifts: { items: [] } } });
  for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
    await page.setViewportSize({ width, height });
    for (const mode of ['light', 'dark'] as const) {
      await page.addInitScript(theme => localStorage.setItem('minipos-web-theme', theme), mode);
      await page.goto('/');
      await expect(page.getByTestId('shift-open')).toBeVisible();
      await expect(page.locator('html')).toHaveAttribute('data-theme', mode);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${size}/${mode}: horizontal overflow`).toBe(true);
      const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
      const screenshot = await page.screenshot({ path: join(evidenceDir, `shift-open-${size}-${mode}.png`), fullPage: true });
      scans.push({ screen: 'shift-open', viewport: size, theme: mode, violations: result.violations, incomplete: result.incomplete, contrastCandidates: await collectContrastCandidates(page, result, screenshot) });
    }
  }
  await writeFile(join(process.cwd(), 'docs', 'ui-optimization-shift-axe.json'), `${JSON.stringify({ scans }, null, 2)}\n`);
  expect(scans.flatMap(scan => (scan.violations as { id: string; nodes: unknown[] }[]).map(violation => ({
    screen: scan.screen, viewport: scan.viewport, theme: scan.theme, id: violation.id, nodes: violation.nodes.length,
  })))).toEqual([]);
});
