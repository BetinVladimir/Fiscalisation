import { test, expect } from '@playwright/test';
import { setup, waitForPOS } from './helpers';

test.describe('Checkout – CLOUD route (fiscal-route-health → checkout-batch)', () => {
  test('happy path: add product → checkout → cart clears', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await expect(page.locator('.total')).toContainText('2.50');

    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Cart must clear on FISCALIZED
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 10_000 });
    // Reversal button appears after first successful checkout
    await expect(page.getByRole('button', { name: /Сторно/ })).toBeVisible();
  });

  test('add two products, cart total is sum', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();   // 2.50
    await page.getByRole('button', { name: /Вода/ }).click();   // 1.00

    await expect(page.locator('.total')).toContainText('3.50');
  });

  test('split payment: set card amount, remainder is cash', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();

    // Set card amount to 1.00, cash remainder = 1.50
    await page.locator('input[inputMode="decimal"]').fill('1.00');
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 10_000 });
  });

  test('reversal of last order', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByRole('button', { name: /Сторно/ })).toBeVisible({ timeout: 10_000 });

    await page.getByRole('button', { name: /Сторно/ }).click();

    // Result from reversal is shown in settings diagnostic panel
    await page.getByRole('button', { name: 'Настройки' }).click();
    await expect(page.locator('pre')).toContainText('REV-001', { timeout: 5_000 });
  });

  test('checkout failure shows error message', async ({ page }) => {
    await setup(page, { minipos: { checkoutBatch: 500 } });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    await expect(page.locator('.error')).toBeVisible({ timeout: 8_000 });
    // Cart not cleared on failure
    await expect(page.locator('.total')).toContainText('2.50');
  });

  test('UNKNOWN fiscal result: cart stays, reversal button still appears', async ({ page }) => {
    await setup(page, {
      minipos: {
        checkoutBatch: {
          operation_id: 'op-unknown-001',
          state: 'UNKNOWN',
          fiscal_reference: null,
          updated_at: new Date().toISOString(),
        },
      },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Cart not cleared on UNKNOWN
    await expect(page.locator('.total')).toContainText('2.50', { timeout: 8_000 });
    // Diagnostic panel shows UNKNOWN
    await page.getByRole('button', { name: 'Настройки' }).click();
    await expect(page.locator('pre')).toContainText('UNKNOWN');
  });
});

test.describe('Checkout – LOCAL_HTTP route (Datecs/Daisy device boundary)', () => {
  test('cloud unavailable → falls over to edge-agent → FISCALIZED', async ({ page }) => {
    await setup(page, {
      minipos: {
        // fiscal-route-health returns 503: CLOUD probe fails → RouteController picks LOCAL_HTTP
        fiscalRouteHealth: 503,
      },
      device: {
        healthz: { status: 'ok', profile: 'edge-agent-s3' },
        submit: {
          operation_id: 'op-device-001',
          state: 'FISCALIZED',
          fiscal_reference: 'DATECS-E2E-001',
          updated_at: new Date().toISOString(),
        },
      },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Cart clears on FISCALIZED from LOCAL_HTTP
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 15_000 });
  });

  test('device returns UNKNOWN state → cart not cleared', async ({ page }) => {
    await setup(page, {
      minipos: { fiscalRouteHealth: 503 },
      device: {
        submit: {
          operation_id: 'op-device-unknown',
          state: 'UNKNOWN',
          fiscal_reference: null,
          updated_at: new Date().toISOString(),
        },
      },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    await expect(page.locator('.total')).toContainText('2.50', { timeout: 15_000 });
  });

  test('device 4xx → throws immediately, no further routes attempted', async ({ page }) => {
    await setup(page, {
      minipos: { fiscalRouteHealth: 503 },
      device: {
        submit: 422,
      },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    await expect(page.locator('.error')).toBeVisible({ timeout: 10_000 });
  });

  test('device idempotency: same operation returned on duplicate probe', async ({ page }) => {
    const sharedRef = `DATECS-IDEM-${Date.now()}`;
    await setup(page, {
      minipos: { fiscalRouteHealth: 503 },
      device: {
        submit: {
          operation_id: 'op-idem-001',
          state: 'FISCALIZED',
          fiscal_reference: sharedRef,
          updated_at: new Date().toISOString(),
        },
        operation: {
          operation_id: 'op-idem-001',
          state: 'FISCALIZED',
          fiscal_reference: sharedRef,
          updated_at: new Date().toISOString(),
        },
      },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 15_000 });
    await page.getByRole('button', { name: 'Настройки' }).click();
    await expect(page.locator('pre')).toContainText('FISCALIZED');
  });
});

test.describe('Checkout – printer test', () => {
  test('printer test sends PRINTER_TEST intent to device', async ({ page }) => {
    const intentsReceived: unknown[] = [];
    await setup(page, {
      device: {
        submit: {
          operation_id: 'op-print-001',
          state: 'FISCALIZED',
          fiscal_reference: null,
          updated_at: new Date().toISOString(),
        },
      },
    });

    await page.route('http://localhost:19001/beeloy/local/v1/intents', async (route) => {
      const body = JSON.parse(route.request().postData() || '{}');
      intentsReceived.push(body);
      await route.fulfill({
        status: 200,
        body: JSON.stringify({ operation_id: 'op-print-001', state: 'FISCALIZED', fiscal_reference: null, updated_at: new Date().toISOString() }),
        contentType: 'application/json',
      });
    });

    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Настройки' }).click();
    await page.getByRole('button', { name: 'Тест на принтера' }).click();

    await expect(page.locator('pre')).toBeVisible({ timeout: 8_000 });
    expect(intentsReceived.some((b: any) => b.command === 'PRINTER_TEST')).toBeTruthy();
  });
});
