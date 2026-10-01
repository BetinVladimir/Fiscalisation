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

test('auth and language menu reflow at 320 CSS px and 200% zoom', async ({ page }) => {
  await setupMiniposRoutes(page);
  for (const [label, width, zoom] of [
    ['320px', 320, 1], ['599px', 599, 1], ['600px', 600, 1],
    ['1023px', 1023, 1], ['1024px', 1024, 1], ['200% CSS zoom', 640, 2],
  ] as const) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto('/');
    if (zoom > 1) await page.addStyleTag({ content: `html { zoom: ${zoom} !important; }` });
    await expect(page.getByTestId('auth-email')).toBeVisible();
    await expectNoHorizontalOverflow(page, `${label}/auth`);
    await page.getByTestId('auth-language-open').click();
    await expect(page.getByTestId('auth-language-ru')).toBeAttached();
    await expectNoHorizontalOverflow(page, `${label}/language menu`);
    await page.keyboard.press('Escape');
  }
});

test('working shell, sale, and management reflow at 320 CSS px and 200% zoom', async ({ page }) => {
  await setup(page);
  for (const [label, width, zoom] of [
    ['320px', 320, 1], ['599px', 599, 1], ['600px', 600, 1],
    ['1023px', 1023, 1], ['1024px', 1024, 1], ['200% CSS zoom', 640, 2],
  ] as const) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto('/');
    if (zoom > 1) await page.addStyleTag({ content: `html { zoom: ${zoom} !important; }` });
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
