import { expect, test } from '@playwright/test';
import { setup, setupMiniposRoutes, waitForPOS } from './helpers';

async function expectNoHorizontalOverflow(page: import('@playwright/test').Page, context: string) {
  const sizes = await page.evaluate(() => ({
    document: document.documentElement.scrollWidth,
    viewport: document.documentElement.clientWidth,
    offenders: [...document.querySelectorAll('*')].filter(element => element.getBoundingClientRect().right > document.documentElement.clientWidth + 1)
      .slice(0, 8).map(element => ({ tag: element.tagName, testId: element.getAttribute('data-testid'), className: String(element.className).slice(0, 100), right: Math.round(element.getBoundingClientRect().right) })),
  }));
  expect(sizes.document, `${context}: document wider than viewport (${sizes.document}/${sizes.viewport}); ${JSON.stringify(sizes.offenders)}`).toBeLessThanOrEqual(sizes.viewport + 1);
}

async function doubleRenderedText(page: import('@playwright/test').Page) {
  await page.evaluate(() => {
    for (const element of document.body.querySelectorAll<HTMLElement>('*')) {
      if (![...element.childNodes].some(node => node.nodeType === Node.TEXT_NODE && node.textContent?.trim())) continue;
      const size = Number.parseFloat(getComputedStyle(element).fontSize);
      if (Number.isFinite(size)) element.style.setProperty('font-size', `${size * 2}px`, 'important');
    }
  });
}

test('auth and language menu reflow at 320 CSS px and breakpoint boundaries', async ({ page }) => {
  await setupMiniposRoutes(page);
  for (const [label, width] of [
    ['320px', 320], ['599px', 599], ['600px', 600],
    ['1023px', 1023], ['1024px', 1024],
  ] as const) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto('/');
    await expect(page.getByTestId('auth-email')).toBeVisible();
    await expectNoHorizontalOverflow(page, `${label}/auth`);
    await page.getByTestId('auth-language-open').click();
    await expect(page.getByTestId('auth-language-ru')).toBeAttached();
    await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    await expectNoHorizontalOverflow(page, `${label}/language menu`);
    await page.keyboard.press('Escape');
  }
});

test('working shell, sale, and management reflow at 320 CSS px and breakpoint boundaries', async ({ page }) => {
  await setup(page);
  for (const [label, width] of [
    ['320px', 320], ['599px', 599], ['600px', 600],
    ['1023px', 1023], ['1024px', 1024],
  ] as const) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto('/');
    await waitForPOS(page);
    await expect(page.getByTestId('sale-payment')).toBeVisible();
    await expectNoHorizontalOverflow(page, `${label}/sale`);
    await page.getByTestId('app-products').click();
    await expect(page.getByTestId('management-list')).toBeVisible();
    await expectNoHorizontalOverflow(page, `${label}/management list`);
    await page.getByTestId('management-add').click();
    await expect(page.getByTestId('management-product-name')).toBeVisible();
    await expectNoHorizontalOverflow(page, `${label}/management form`);
  }
});

test('rendered auth and sale text can double without document overflow', async ({ page }) => {
  await page.setViewportSize({ width: 640, height: 800 });
  await setupMiniposRoutes(page);
  await page.goto('/');
  await expect(page.getByTestId('auth-email')).toBeVisible();
  await doubleRenderedText(page);
  await expectNoHorizontalOverflow(page, 'auth/200% rendered text');

  await setup(page);
  await page.goto('/');
  await waitForPOS(page);
  await doubleRenderedText(page);
  await expectNoHorizontalOverflow(page, 'sale/200% rendered text');
});
