import { writeFile } from 'node:fs/promises';
import { test, expect, type Locator } from '@playwright/test';
import { doubleText } from './text-scale';
import { frameContrast } from './affordance-contrast';
import { mockFiscalApi, openApp } from './helpers';

async function expectFrame(action: Locator) {
  await action.scrollIntoViewIfNeeded();
  await expect.poll(() => action.evaluate(element => {
    for (let node: Element | null = element; node; node = node.parentElement) {
      if (Number(getComputedStyle(node).opacity) !== 1) return false;
    }
    return true;
  }), { message: 'Measure only after opening animation settles' }).toBe(true);
  const frame = await action.evaluate(element => {
    const style = getComputedStyle(element);
    const rect = element.getBoundingClientRect();
    return { width: parseFloat(style.borderTopWidth), style: style.borderTopStyle,
      color: style.borderTopColor, height: rect.height, left: rect.left, right: rect.right,
      viewport: innerWidth };
  });
  let textMetrics: { text: string | null; width: number; scroll: number; original: number; size: number }[] = [];
  if (process.env.TEXT200 === '1') {
    const labels = await action.locator('[dir="auto"]').evaluateAll(nodes => nodes.map(node => ({
      text: node.textContent, width: node.clientWidth, scroll: node.scrollWidth,
      original: Number((node as HTMLElement).dataset.originalFontSize), size: parseFloat(getComputedStyle(node).fontSize),
    })));
    textMetrics = labels;
    expect(labels.length).toBeGreaterThan(0);
    for (const label of labels) {
      expect(label.size).toBe(label.original * 2);
      expect(label.scroll, `Clipped label: ${label.text}`).toBeLessThanOrEqual(label.width + 1);
    }
  }
  const contrast = await frameContrast(action);
  expect(contrast.insideRatio, 'Frame against control fill').toBeGreaterThanOrEqual(3);
  expect(contrast.outsideRatio, 'Frame against surrounding surface').toBeGreaterThanOrEqual(3);
  expect(frame.width).toBeGreaterThanOrEqual(1);
  expect(frame.style).toBe('solid');
  expect(frame.color).not.toBe('rgba(0, 0, 0, 0)');
  expect(frame.height).toBeGreaterThanOrEqual(48);
  expect(frame.left).toBeGreaterThanOrEqual(0);
  expect(frame.right).toBeLessThanOrEqual(frame.viewport + 1);
  return { ...contrast, textMetrics };
}

async function expectFocus(action: Locator) {
  const focus = await action.evaluate(element => {
    const s = getComputedStyle(element);
    return { visible: element.matches(':focus-visible'), width: parseFloat(s.outlineWidth),
      style: s.outlineStyle, color: s.outlineColor };
  });
  expect(focus.visible).toBe(true);
  expect(focus.width).toBeGreaterThan(0);
  expect(focus.style).not.toBe('none');
  const contrast = await frameContrast(action, 'outlineColor');
  expect(contrast.outsideRatio, 'Focus against surrounding surface').toBeGreaterThanOrEqual(3);
  return { ...focus, contrast };
}

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Paper navigation and reference affordance ${size}/${theme}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      const mutations = await mockFiscalApi(page, { secondRegister: true });
      await openApp(page);
      if (process.env.TEXT200 === '1') await doubleText(page);
      const reference = page.getByRole('button', { name: 'R02', exact: true }).first();
      const referenceContrast = await expectFrame(reference);
      await page.keyboard.press('Tab');
      await reference.focus();
      const referenceFocus = await expectFocus(reference);
      await page.keyboard.press('Enter');
      await expect(page.getByRole('button', { name: 'Избрано: R02', exact: true }).first()).toBeVisible();
      if (size === 'xs') {
        await page.getByTestId('nav.open').focus();
        await page.keyboard.press('Enter');
      }
      const navigation = page.getByRole('button', { name: 'Отвори Администриране', exact: true });
      const navigationContrast = await expectFrame(navigation);
      if (size !== 'xs') { const rect = await navigation.boundingBox(); expect(rect!.x + rect!.width).toBeLessThanOrEqual(191); }
      await navigation.focus();
      const navigationFocus = await expectFocus(navigation);
      await page.keyboard.press('Space');
      await expect(page.getByTestId('admin-editors')).toBeVisible();
      const choice = page.getByRole('toolbar', { name: 'Съществуващи записи' }).getByRole('button', { name: 'Избрано: SOF • Sofia', exact: true });
      const choiceContrast = await expectFrame(choice);
      await page.getByTestId('admin-location-code').fill('CHANGED');
      await choice.focus();
      const choiceFocus = await expectFocus(choice);
      await page.keyboard.press('Enter');
      await expect(page.getByTestId('admin-location-code')).toHaveValue('SOF');
      await expect(page.getByRole('button', { name: 'Избрано: SOF • Sofia', exact: true })).toBeVisible();
      await writeFile(testInfo.outputPath('affordance-contrast.json'), JSON.stringify({ size, theme, referenceContrast, navigationContrast, choiceContrast, referenceFocus, navigationFocus, choiceFocus }, null, 2));
      await testInfo.attach('affordance-contrast', { body: JSON.stringify({ size, theme, referenceContrast, navigationContrast, choiceContrast, referenceFocus, navigationFocus, choiceFocus }), contentType: 'application/json' });
      if (process.env.TEXT200 === '1') await page.screenshot({ path: testInfo.outputPath('text200.png') });
      expect(mutations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}


test('contrast negative control detects a frame matching the fill', async ({ page }) => {
  await mockFiscalApi(page, { secondRegister: true });
  await openApp(page);
  const action = page.getByRole('button', { name: 'R02', exact: true }).first();
  const original = await action.getAttribute('style');
  await action.evaluate(element => { (element as HTMLElement).style.borderColor = getComputedStyle(element).backgroundColor; });
  try {
    const contrast = await frameContrast(action);
    expect(contrast.insideRatio).toBe(1);
    expect(contrast.outsideRatio).toBeLessThan(3);
  } finally {
    await action.evaluate((element, style) => { if (style === null) element.removeAttribute('style'); else element.setAttribute('style', style); }, original);
  }
  await expectFrame(action);
});
