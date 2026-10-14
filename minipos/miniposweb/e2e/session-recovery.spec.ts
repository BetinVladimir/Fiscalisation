import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { setup } from './helpers';
import { translations } from '../src/ui/workTranslations.generated';
import { collectContrastCandidates } from './contrast-candidates';

test('session recovery offers localized retry, theme, language and logout', async ({ page }) => {
  let sessionRequests = 0;
  await setup(page, { minipos: { session: 503 } });
  page.on('request', request => {
    if (request.url().includes('/operator-session')) sessionRequests++;
  });
  await page.goto('/');
  await expect(page.getByTestId('session-status')).toHaveText(translations.bg.sessionUnavailable);
  await page.getByTestId('session-retry').click();
  await expect.poll(() => sessionRequests).toBeGreaterThanOrEqual(2);
  await expect(page.getByTestId('session-status')).toHaveText(translations.bg.sessionUnavailable);

  await page.getByTestId('session-language-open').click();
  await page.getByTestId('session-language-ru').scrollIntoViewIfNeeded();
  await page.getByTestId('session-language-ru').click();
  await expect(page.getByTestId('session-status')).toHaveText(translations.ru.sessionUnavailable);
  await page.getByTestId('session-theme-open').click();
  await page.getByTestId('session-theme-dark').click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.getByTestId('session-logout').click();
  await expect(page.getByTestId('auth-email')).toBeVisible();
});

test('session loading and error: xs/md/lg × light/dark WCAG matrix', async ({ page }) => {
  test.setTimeout(120_000);
  await setup(page, { minipos: { session: 503 } });
  const evidenceDir = join(process.cwd(), 'docs', 'ui-screenshots');
  await mkdir(evidenceDir, { recursive: true });
  const scans: {screen:string;viewport:string;theme:string;violations:Awaited<ReturnType<AxeBuilder['analyze']>>['violations'];incomplete:Awaited<ReturnType<AxeBuilder['analyze']>>['incomplete'];contrastCandidates:unknown[]}[] = [];
  for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
    await page.setViewportSize({ width, height });
    for (const theme of ['light', 'dark'] as const) {
      await page.addInitScript(value => localStorage.setItem('minipos-web-theme', value), theme);
      let release: (() => void) | undefined;
      const pattern = '**/public/v1/minipos/operator-session**';
      await page.route(pattern, async route => {
        await new Promise<void>(resolve => { release = resolve; });
        await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'UNAVAILABLE' }) });
      });
      await page.goto('/');
      await expect.poll(() => Boolean(release)).toBe(true);
      await expect(page.getByTestId('session-status')).toHaveText(translations.bg.loading);
      await expect(page.getByTestId('session-retry')).toBeDisabled();
      await scan('loading');
      release?.();
      await expect(page.getByTestId('session-status')).toHaveText(translations.bg.sessionUnavailable);
      await scan('error');
      await page.unroute(pattern);

      async function scan(state: string) {
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
      await expect(page.getByTestId('session-language-open')).toBeVisible();
      await expect(page.getByTestId('session-theme-open')).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${size}/${theme}: overflow`).toBe(true);
      const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
      const screenshot = await page.screenshot({ path: join(evidenceDir, `session-recovery-${state}-${size}-${theme}.png`), fullPage: true });
      scans.push({ screen: `session-recovery-${state}`, viewport: size, theme, violations: result.violations, incomplete: result.incomplete,
        contrastCandidates: await collectContrastCandidates(page, result, screenshot) });
      }
    }
  }
  expect(scans.flatMap(scan=>scan.incomplete),'Incomplete requires resolution').toEqual([]);
  await writeFile(join(process.cwd(), 'docs', 'ui-optimization-session-recovery-axe.json'), `${JSON.stringify({ scans }, null, 2)}\n`);
  expect(scans.flatMap(scan => scan.violations.map(violation => ({ viewport: scan.viewport, theme: scan.theme, id: violation.id })))).toEqual([]);
});
