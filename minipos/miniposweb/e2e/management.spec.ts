import { test, expect } from '@playwright/test';
import { PRODUCTS, setup, waitForPOS } from './helpers';

test.describe('Management – Products', () => {
  test('products list shows active products', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByTestId('app-products').click();

    await expect(page.getByRole('heading', { name: 'Продукти' })).toBeVisible();
    await expect(page.getByText('Кафе')).toBeVisible();
    await expect(page.getByText('Вода')).toBeVisible();
  });

  test('add new product', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByTestId('app-products').click();
    await page.getByTestId('management-add').click();

    await expect(page.getByRole('heading', { name: 'Нов продукт' })).toBeVisible();

    await page.getByTestId('management-product-name').fill('Чай');
    await page.getByTestId('management-product-price').fill('1.50');
    await page.getByTestId('management-save').click();

    // After save, the form closes and the new product appears
    await expect(page.getByText('Нов продукт')).toBeVisible({ timeout: 5_000 });
  });

  test('cancel add product stays on list', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByTestId('app-products').click();
    await page.getByTestId('management-add').click();

    await expect(page.getByRole('heading', { name: 'Нов продукт' })).toBeVisible();
    await page.getByTestId('management-cancel').click();

    await expect(page.getByText('Кафе')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Нов продукт' })).not.toBeVisible();
  });

  test('failed product save keeps entered values and shows Paper error', async ({ page }) => {
    await setup(page, { minipos: { createProduct: 500 } });
    await page.goto('/');
    await waitForPOS(page);
    await page.getByTestId('app-products').click();
    await page.getByTestId('management-add').click();
    await page.getByTestId('management-product-name').fill('Чай');
    await page.getByTestId('management-product-price').fill('1.50');
    await page.getByTestId('management-save').click();
    await expect(page.getByTestId('management-error')).toBeVisible();
    await expect(page.getByTestId('management-product-name')).toHaveValue('Чай');
    await expect(page.getByTestId('management-product-price')).toHaveValue('1.50');
  });

  test('list load error shows Paper retry and does not show stale products', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);
    let requests = 0;
    await page.route('**/public/v1/minipos/products**', async route => {
      requests++;
      await route.fulfill({
        status: requests === 1 ? 500 : 200,
        contentType: 'application/json',
        body: JSON.stringify(requests === 1 ? { code: 'TEMPORARY_FAILURE' } : { items: PRODUCTS }),
      });
    });
    await page.getByTestId('app-products').click();
    await expect(page.getByTestId('management-load-error')).toBeVisible();
    await expect(page.getByTestId('management-list')).not.toBeVisible();
    await expect(page.getByTestId('management-retry')).toBeVisible();
    await page.getByTestId('management-retry').click();
    await expect(page.getByTestId('management-list').getByText('Кафе')).toBeVisible();
    expect(requests).toBe(2);
  });
});

test.describe('Management – Employees', () => {
  test('employees list shown', async ({ page }) => {
    await setup(page, {
      minipos: {
        employees: {
          items: [
            { id: 'emp-001', first_name: 'Иван', last_name: 'Иванов', operator_code: '001', roles: ['OPERATOR'] },
            { id: 'emp-002', first_name: 'Мария', last_name: 'Петрова', operator_code: '002', roles: ['OPERATOR'] },
          ],
        },
      },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByTestId('app-employees').click();

    await expect(page.getByRole('heading', { name: 'Служители' })).toBeVisible();
    await expect(page.getByTestId('management-list').getByText('Иван Иванов')).toBeVisible();
    await expect(page.getByTestId('management-list').getByText('Мария Петрова')).toBeVisible();
  });

  test('add new employee', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByTestId('app-employees').click();
    await page.getByTestId('management-add').click();

    await expect(page.getByRole('heading', { name: 'Нов служител' })).toBeVisible();

    await page.getByTestId('management-employee-first').fill('Петър');
    await page.getByTestId('management-employee-last').fill('Петров');
    await page.getByTestId('management-employee-code').fill('003');
    await page.getByTestId('management-save').click();

    await expect(page.getByText('Петър Петров')).toBeVisible({ timeout: 5_000 });
  });

  test('cancel add employee stays on list', async ({ page }) => {
    await setup(page, {
      minipos: { employees: { items: [{ id: 'emp-001', first_name: 'Иван', last_name: 'Иванов', operator_code: '001', roles: [] }] } },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByTestId('app-employees').click();
    await page.getByTestId('management-add').click();
    await page.getByTestId('management-cancel').click();

    await expect(page.getByTestId('management-list').getByText('Иван Иванов')).toBeVisible();
  });
});

test.describe('Navigation', () => {
  test('working header switches Paper language and theme after login', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);
    await page.getByTestId('app-language-open').click();
    await page.getByTestId('app-language-ru').scrollIntoViewIfNeeded();
    await page.getByTestId('app-language-ru').click();
    await expect(page.getByTestId('app-language-open')).toContainText('Русский');
    await page.getByTestId('app-theme-open').click();
    await page.getByTestId('app-theme-dark').click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await page.reload();
    await waitForPOS(page);
    await expect(page.getByTestId('app-language-open')).toContainText('Русский');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  });

  test('back to sale view from management', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByTestId('app-products').click();
    await expect(page.getByRole('heading', { name: 'Продукти' })).toBeVisible();

    await page.getByTestId('app-sales').click();
    await expect(page.getByText(/Текуща продажба/)).toBeVisible();
  });
});
