import { test, expect } from '@playwright/test';
import { setup, setupMiniposRoutes, setupDeviceRoutes, authenticate, waitForPOS } from './helpers';

test.describe('RouteController – route selection and failover', () => {
  test('CLOUD preferred by default: checkout-batch called when health is ok', async ({ page }) => {
    let checkoutBatchCalled = false;
    let deviceIntentsCalled = false;

    await setupMiniposRoutes(page, { fiscalRouteHealth: { status: 'ok' } });
    await setupDeviceRoutes(page);
    await authenticate(page);

    await page.route('**/checkout-batch', async (route) => {
      checkoutBatchCalled = true;
      return route.fulfill({
        status: 200,
        body: JSON.stringify({ operation_id: 'op-cloud-001', state: 'FISCALIZED', fiscal_reference: 'CLOUD-001', updated_at: new Date().toISOString() }),
        contentType: 'application/json',
      });
    });
    await page.route('http://localhost:19001/**/intents', async (route) => {
      deviceIntentsCalled = true;
      await route.continue();
    });

    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 10_000 });

    expect(checkoutBatchCalled).toBe(true);
    expect(deviceIntentsCalled).toBe(false);
  });

  test('fiscal-route-health 5xx → RouteController falls to LOCAL_HTTP device', async ({ page }) => {
    let checkoutBatchCalled = false;
    let deviceIntentsCalled = false;

    await setupMiniposRoutes(page, { fiscalRouteHealth: 503 });
    await setupDeviceRoutes(page, {
      submit: {
        operation_id: 'op-device-001',
        state: 'FISCALIZED',
        fiscal_reference: 'DATECS-001',
        updated_at: new Date().toISOString(),
      },
    });
    await authenticate(page);

    await page.route('**/checkout-batch', async (route) => {
      checkoutBatchCalled = true;
      await route.continue();
    });
    await page.route('http://localhost:19001/**/intents', async (route) => {
      deviceIntentsCalled = true;
      await route.fulfill({
        status: 200,
        body: JSON.stringify({ operation_id: 'op-device-001', state: 'FISCALIZED', fiscal_reference: 'DATECS-001', updated_at: new Date().toISOString() }),
        contentType: 'application/json',
      });
    });

    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 15_000 });

    expect(checkoutBatchCalled).toBe(false);
    expect(deviceIntentsCalled).toBe(true);
  });

  test('4xx from device → throws immediately without next route', async ({ page }) => {
    const errors: string[] = [];

    await setupMiniposRoutes(page, { fiscalRouteHealth: 503 });
    await setupDeviceRoutes(page, { submit: 422 });
    await authenticate(page);

    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    await expect(page.locator('.error')).toBeVisible({ timeout: 10_000 });
    // Cart not cleared
    await expect(page.locator('.total')).toContainText('2.50');
  });

  test('order creation fails → CLOUD route skipped, only LOCAL_HTTP active', async ({ page }) => {
    let checkoutBatchCalled = false;

    await setupMiniposRoutes(page, { createOrder: 503 });
    await setupDeviceRoutes(page, {
      submit: {
        operation_id: 'op-device-offline-001',
        state: 'FISCALIZED',
        fiscal_reference: 'DATECS-OFFLINE-001',
        updated_at: new Date().toISOString(),
      },
    });
    await authenticate(page);

    await page.route('**/checkout-batch', async (route) => {
      checkoutBatchCalled = true;
      await route.continue();
    });

    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Device path succeeds even without cloud order
    await expect(page.getByText(/Cloud подготовката е недостъпна/)).toBeVisible({ timeout: 10_000 });
    // checkout-batch never called (no order to attach it to)
    expect(checkoutBatchCalled).toBe(false);
  });

  test('all routes fail: NO_FISCAL_ROUTE error shown', async ({ page }) => {
    await setupMiniposRoutes(page, { fiscalRouteHealth: 503 });
    // Device returns 500 → non-4xx, observe(false), no further routes
    await setupDeviceRoutes(page, { healthz: 500 });
    await authenticate(page);

    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    await expect(page.locator('.error')).toBeVisible({ timeout: 10_000 });
  });
});

test.describe('RouteController – cloud success recovery', () => {
  test('three consecutive CLOUD successes restore CLOUD preference', async ({ page }) => {
    let checkoutCallCount = 0;

    await setup(page, { minipos: { fiscalRouteHealth: { status: 'ok' } } });

    await page.route('**/checkout-batch', async (route) => {
      checkoutCallCount++;
      return route.fulfill({
        status: 200,
        body: JSON.stringify({ operation_id: `op-cloud-${checkoutCallCount}`, state: 'FISCALIZED', fiscal_reference: `CLOUD-${checkoutCallCount}`, updated_at: new Date().toISOString() }),
        contentType: 'application/json',
      });
    });

    await page.goto('/');
    await waitForPOS(page);

    // Three checkouts to drive cloudSuccesses counter to recoveryThreshold (3)
    for (let i = 0; i < 3; i++) {
      await page.getByRole('button', { name: /Кафе/ }).click();
      await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
      await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 10_000 });
    }

    expect(checkoutCallCount).toBe(3);
  });
});
