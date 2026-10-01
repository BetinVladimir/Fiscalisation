import { test, expect } from '@playwright/test';
import { setup, waitForPOS } from './helpers';
import { supportedLanguages, translations } from '../src/ui/workTranslations.generated';

test('all 25 languages update sale and navigation without fallback keys', async ({ page }) => {
  test.setTimeout(120_000);
  await setup(page);
  await page.setViewportSize({ width: 360, height: 800 });
  await page.goto('/');
  await waitForPOS(page);
  for (const language of supportedLanguages) {
    await page.getByTestId('app-language-open').click();
    await page.getByTestId(`app-language-${language}`).scrollIntoViewIfNeeded();
    await page.getByTestId(`app-language-${language}`).click();
    const t = translations[language];
    await expect(page.getByTestId('app-sales')).toContainText(t.sales);
    await expect(page.getByTestId('app-products')).toContainText(t.products);
    await expect(page.getByTestId('app-employees')).toContainText(t.employees);
    await expect(page.getByTestId('sale-payment').getByRole('heading', { name: t.currentSale })).toBeVisible();
    await expect(page.getByTestId('sale-checkout')).toContainText(t.checkoutFiscal);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `overflow in ${language}`).toBe(true);
  }
  await page.reload();
  await waitForPOS(page);
  await expect(page.getByTestId('app-sales')).toContainText(translations.ru.sales);
});

test('EN/BG/RU/DE/FR update management forms and labels', async ({ page }) => {
  test.setTimeout(90_000);
  await setup(page);
  await page.setViewportSize({ width: 360, height: 800 });
  await page.goto('/');
  await waitForPOS(page);
  for (const language of ['en', 'bg', 'ru', 'de', 'fr'] as const) {
    await page.getByTestId('app-language-open').click();
    await page.getByTestId(`app-language-${language}`).scrollIntoViewIfNeeded();
    await page.getByTestId(`app-language-${language}`).click();
    const t = translations[language];
    await page.getByTestId('app-products').click();
    await expect(page.getByRole('heading', { name: t.products })).toBeVisible();
    await page.getByTestId('management-add').click();
    await expect(page.getByRole('heading', { name: t.newProduct })).toBeVisible();
    await expect(page.getByTestId('management-product-name')).toHaveAttribute('aria-label', t.name);
    await expect(page.getByTestId('management-product-price')).toHaveAttribute('aria-label', t.priceEur);
    await page.getByTestId('management-cancel').click();
    await page.getByTestId('app-employees').click();
    await expect(page.getByRole('heading', { name: t.employees })).toBeVisible();
    await page.getByTestId('management-add').click();
    await expect(page.getByRole('heading', { name: t.newEmployee })).toBeVisible();
    await expect(page.getByTestId('management-employee-code')).toHaveAttribute('aria-label', t.operatorCode);
    await page.getByTestId('management-cancel').click();
    await page.getByTestId('app-sales').click();
    await expect(page.getByTestId('sale-payment')).toBeVisible();
  }
});
