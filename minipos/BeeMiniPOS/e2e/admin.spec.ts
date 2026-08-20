import { test, expect } from '@playwright/test';
import { setup, waitForReady } from './helpers';

test.describe('Admin panel – navigation', () => {
  test('admin-toggle shows admin panel, toggles back to sales', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });

    await page.getByTestId('admin-toggle').click();
    await expect(page.getByTestId('admin-editors')).not.toBeVisible();
    await expect(page.getByTestId('product-coffee')).toBeVisible();
  });

  test('admin shows products, tax-groups, employees shortcuts', async ({ page }) => {
    await setup(page, { minipos: { taxGroups: { items: [{ id: 'tg-b', code: 'B', name: 'Стандартна', rate: '20', status: 'ACTIVE', version: 1 }], page: { has_more: false } } } });
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });

    await expect(page.getByTestId('admin-editors')).toContainText('Продукт');
    await expect(page.getByTestId('admin-editors')).toContainText('ДДС групи');
    await expect(page.getByTestId('admin-editors')).toContainText('Служите');
  });
});

test.describe('Admin panel – product catalog', () => {
  test('navigate to products list', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });
    await page.getByTestId('admin-products').click();

    await expect(page.getByText('Кафе')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Вода')).toBeVisible();
  });

  test('create new product via + button', async ({ page }) => {
    let productCreated = false;
    await setup(page);

    await page.route('**/public/v1/minipos/products', async (route) => {
      if (route.request().method() === 'POST') {
        productCreated = true;
        return route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'prod-new', sku: 'TEA', name: 'Чай', price: { amount: '1.80', currency: 'EUR' }, tax_group: 'B', active: true }),
        });
      }
      return route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });
    await page.getByTestId('admin-products').click();
    await page.getByTestId('product-add').click();

    // New product form – find by placeholder
    await page.getByTestId('product-name').fill('Чай');
    await page.getByTestId('product-barcode').fill('3800000000099');
    await page.getByTestId('product-price').fill('1.80');

    // Select tax group B if shown
    const taxGroupBtn = page.locator('[role="button"]').filter({ hasText: /^B •/ });
    if (await taxGroupBtn.count() > 0) await taxGroupBtn.click();

    await page.getByTestId('product-create').click();

    expect(productCreated).toBe(true);
    await expect(page.getByTestId('product-add')).toBeVisible();
  });
});

test.describe('Admin panel – employee catalog', () => {
  test('navigate to employees list and add new employee', async ({ page }) => {
    let employeeCreated = false;
    await setup(page);

    await page.route('**/public/v1/minipos/employees', async (route) => {
      if (route.request().method() === 'POST') {
        employeeCreated = true;
        const body = JSON.parse(route.request().postData() || '{}');
        return route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'emp-2', first_name: body.first_name, last_name: body.last_name, operator_code: body.operator_code }),
        });
      }
      return route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });
    await page.getByTestId('admin-employees').click();

    await expect(page.getByText('Иван Петров', { exact: true })).toBeVisible({ timeout: 5_000 });
    await page.getByTestId('employee-add').click();

    await page.getByTestId('employee-first-name').fill('Мария');
    await page.getByTestId('employee-last-name').fill('Иванова');
    await page.getByTestId('employee-code').fill('0002');
    await page.getByTestId('employee-create').click();

    expect(employeeCreated).toBe(true);
    await expect(page.getByTestId('employee-add')).toBeVisible();
  });
});

test.describe('Admin panel – tax groups', () => {
  test('navigate to tax groups, create new group', async ({ page }) => {
    let taxCreated = false;
    await setup(page);

    await page.route('**/public/v1/minipos/tax-groups', async (route) => {
      if (route.request().method() === 'POST') {
        taxCreated = true;
        return route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'tg-a', code: 'A', name: 'Намалена', rate: '9', status: 'ACTIVE', version: 1 }),
        });
      }
      return route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });
    await page.getByTestId('admin-tax-groups').click();

    await expect(page.getByText('B • Стандартна')).toBeVisible({ timeout: 5_000 });
    await page.getByTestId('tax-group-add').click();

    await page.getByTestId('tax-group-code').fill('A');
    await page.getByTestId('tax-group-name').fill('Намалена');
    await page.getByTestId('tax-group-rate').fill('9');
    await page.getByTestId('tax-group-save').click();

    expect(taxCreated).toBe(true);
  });
});

test.describe('Admin panel – configuration', () => {
  test('update location name and save', async ({ page }) => {
    let savedConfig: unknown = null;
    await setup(page, { minipos: { shifts: { items: [], page: { has_more: false } } } });

    await page.route('**/public/v1/minipos/configuration', async (route) => {
      if (route.request().method() === 'PATCH') {
        savedConfig = JSON.parse(route.request().postData() || '{}');
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'config-1', location_name: (savedConfig as any).location_name, location_address: '', workstation_name: 'Каса 01', fiscal_register_id: '00000000-0000-4000-8000-000000000001', version: 2 }),
        });
      }
      return route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });

    await page.getByTestId('config-location-name').clear();
    await page.getByTestId('config-location-name').fill('Магазин София');
    await page.getByTestId('config-save').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /записани/ })
      .waitFor({ timeout: 8_000 });

    expect((savedConfig as any)?.location_name).toBe('Магазин София');
  });

  test('cannot save configuration while shift is open', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });

    await page.getByTestId('config-save').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Затворете смяната/ })
      .waitFor({ timeout: 5_000 });
  });
});

test.describe('Admin panel – sales report', () => {
  test('load report shows gross total', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });

    await page.getByTestId('sales-report-load').click();
    await page.getByTestId('sales-report-result').waitFor({ timeout: 8_000 });
    await expect(page.getByTestId('sales-report-result')).toContainText('250.00');
  });

  test('report request includes valid RFC 3339 date range', async ({ page }) => {
    let reportUrl: string | null = null;
    await setup(page);
    await page.route('**/reports/sales**', async (route) => {
      reportUrl = route.request().url();
      await route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });
    await page.getByTestId('sales-report-load').click();
    await page.getByTestId('sales-report-result').waitFor({ timeout: 8_000 });

    expect(reportUrl).not.toBeNull();
    const url = new URL(reportUrl!);
    const from = Date.parse(url.searchParams.get('from') || '');
    const to = Date.parse(url.searchParams.get('to') || '');
    expect(Number.isFinite(from)).toBe(true);
    expect(Number.isFinite(to)).toBe(true);
    expect(from).toBeLessThan(to);
  });
});

test.describe('Admin panel – reversals', () => {
  test('completed order shows reversal button, reversal succeeds', async ({ page }) => {
    let reversalCalled = false;
    await setup(page, {
      minipos: {
        orders: {
          items: [{
            id: 'order-completed-1',
            state: 'COMPLETED',
            version: 3,
            allowed_actions: ['REVERSE'],
            fiscal_operation_id: 'fiscal-1',
            receipt_reference: 'REF-001',
          }],
          page: { has_more: false },
        },
      },
    });

    await page.route('**/orders/order-completed-1/reversals', async (route) => {
      reversalCalled = true;
      return route.fulfill({
        status: 202,
        contentType: 'application/json',
        body: JSON.stringify({ operation_id: 'rev-op-1', state: 'FISCALIZED', fiscal_reference: 'REV-001' }),
      });
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('admin-toggle').click();
    await page.getByTestId('admin-editors').waitFor({ timeout: 5_000 });

    await page.getByTestId('reversal-order-completed-1').click();
    await page.getByTestId('reversal-editor')
      .filter({ hasText: /REV-001/ })
      .waitFor({ timeout: 8_000 });

    expect(reversalCalled).toBe(true);
  });
});
