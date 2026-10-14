import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { setup, waitForPOS } from './helpers';
import { collectContrastCandidates } from './contrast-candidates';
import { assertResolvedContrast } from './contrast-fallback';

test('management lists and forms: xs/md/lg × light/dark WCAG matrix', async ({ page }) => {
  test.setTimeout(180_000);
  const evidenceDir = join(process.cwd(), 'docs', 'ui-screenshots');
  await mkdir(evidenceDir, { recursive: true });
  const scans: { screen: string; viewport: string; theme: string; violations: unknown[]; incomplete: unknown[]; contrastCandidates: unknown[] }[] = [];
  await setup(page, { minipos: { employees: { items: [
    { id: 'emp-001', first_name: 'Иван', last_name: 'Иванов', operator_code: '001', roles: ['OPERATOR'] },
  ] } } });
  for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
    await page.setViewportSize({ width, height });
    for (const mode of ['light', 'dark'] as const) {
      await page.addInitScript(theme => localStorage.setItem('minipos-web-theme', theme), mode);
      await page.goto('/');
      await waitForPOS(page);
      await expect(page.locator('html')).toHaveAttribute('data-theme', mode);
      for (const [screen, nav, title] of [
        ['products', 'app-products', 'Продукти'],
        ['employees', 'app-employees', 'Служители'],
      ] as const) {
        await page.getByTestId(nav).click();
        await expect(page.getByRole('heading', { name: title })).toBeVisible();
        await expect(page.getByTestId('management-list')).toBeVisible();
        await scan(`${screen}-list`);
        await page.getByTestId('management-add').click();
        await expect(page.getByTestId(screen === 'products' ? 'management-product-name' : 'management-employee-first')).toBeVisible();
        await scan(`${screen}-form`);
        await page.getByTestId('management-cancel').click();
      }

      async function scan(screen: string) {
        const overflows = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1);
        expect(overflows, `${screen}/${size}/${mode}: horizontal overflow`).toBe(false);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
        await assertResolvedContrast(page,result);
        const screenshot = await page.screenshot({ path: join(evidenceDir, `management-${screen}-${size}-${mode}.png`), fullPage: true });
        const contrastCandidates = await collectContrastCandidates(page, result, screenshot);
        scans.push({ screen, viewport: size, theme: mode, violations: result.violations, incomplete: result.incomplete, contrastCandidates });
      }
    }
  }
  await writeFile(join(process.cwd(), 'docs', 'ui-optimization-management-axe.json'), `${JSON.stringify({ scans }, null, 2)}\n`);
  expect(scans.flatMap(scan => (scan.violations as { id: string; nodes: unknown[] }[]).map(violation => ({
    screen: scan.screen, viewport: scan.viewport, theme: scan.theme, id: violation.id, nodes: violation.nodes.length,
  })))).toEqual([]);
});

test('management loading and load-error: xs/md/lg × light/dark', async ({ page }) => {
  test.setTimeout(180_000);
  const evidenceDir = join(process.cwd(), 'docs', 'ui-screenshots');
  await mkdir(evidenceDir, { recursive: true });
  const scans: { screen: string; viewport: string; theme: string; violations: unknown[]; incomplete: unknown[]; contrastCandidates: unknown[] }[] = [];
  await setup(page);
  for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
    await page.setViewportSize({ width, height });
    for (const mode of ['light', 'dark'] as const) {
      await page.addInitScript(theme => localStorage.setItem('minipos-web-theme', theme), mode);
      await page.goto('/');
      await waitForPOS(page);
      let release: (() => void) | undefined;
      const pattern = '**/public/v1/minipos/products**';
      await page.route(pattern, async route => {
        await new Promise<void>(resolve => { release = resolve; });
        await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ code: 'TEMPORARY_FAILURE' }) });
      });
      await page.getByTestId('app-products').click();
      await expect(page.getByTestId('management-loading')).toBeVisible();
      await scan('management-loading');
      await expect.poll(() => Boolean(release)).toBe(true);
      release?.();
      await expect(page.getByTestId('management-load-error')).toBeVisible();
      await scan('management-load-error');
      await page.unroute(pattern);

      async function scan(screen: string) {
        await expect(page.locator('html')).toHaveAttribute('data-theme', mode);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${screen}/${size}/${mode}: horizontal overflow`).toBe(true);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
        await assertResolvedContrast(page,result);
        const screenshot = await page.screenshot({ path: join(evidenceDir, `${screen}-${size}-${mode}.png`), fullPage: true });
        const contrastCandidates = await collectContrastCandidates(page, result, screenshot);
        scans.push({ screen, viewport: size, theme: mode, violations: result.violations, incomplete: result.incomplete, contrastCandidates });
      }
    }
  }
  await writeFile(join(process.cwd(), 'docs', 'ui-optimization-management-states-axe.json'), `${JSON.stringify({ scans }, null, 2)}\n`);
  expect(scans.flatMap(scan => (scan.violations as { id: string; nodes: unknown[] }[]).map(violation => ({
    screen: scan.screen, viewport: scan.viewport, theme: scan.theme, id: violation.id, nodes: violation.nodes.length,
  })))).toEqual([]);
});
