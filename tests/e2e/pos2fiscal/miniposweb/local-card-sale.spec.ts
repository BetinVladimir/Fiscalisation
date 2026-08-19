/**
 * miniposweb – LOCAL_HTTP route, CARD sale via bluecash-app (Datecs BlueCash-50 boundary)
 *
 * When the adapter_base_url points to the bluecash-app device mock (port 19002) and CLOUD
 * is unavailable, the browser calls the card terminal directly via the edge-agent local API:
 *   GET  http://localhost:19002/beeloy/local/v1/healthz
 *   POST http://localhost:19002/beeloy/local/v1/intents
 *
 * The bluecash-app profile validates that payment can go through the card terminal and adds
 * rrn, authorization_code, payment_reference to the operation result.
 *
 * POS terminal control: minipos-backend routes mocked; card payment amount captured from intent.
 * Fiscal device control: bluecash-app device mock on port 19002 is the real Datecs boundary.
 */

import { test, expect } from '@playwright/test';
import {
  setupMinipowsebRoutes,
  authenticate,
  waitForPOS,
  getLatestDeviceOperation,
  BLUECASH_APP_BASE,
  BLUECASH_APP_ORIGIN,
} from '../helpers';

test.describe('miniposweb – LOCAL_HTTP card sale (bluecash-app)', () => {
  test('full CARD checkout via bluecash-app → FISCALIZED with rrn and authorization_code', async ({ page }) => {
    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      // Adapter URL points to bluecash-app (card terminal) mock
      adapterBase: BLUECASH_APP_BASE,
    });
    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await expect(page.locator('.total')).toContainText('2.50');

    // Enter full amount as card → payment type CARD
    await page.locator('input[inputMode="decimal"]').fill('2.50');
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Cart clears on FISCALIZED
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    // ── Fiscal device audit: bluecash-app received and processed the intent ──
    const op = await getLatestDeviceOperation(19002);
    expect(op).toBeDefined();

    expect(op!.state).toBe('FISCALIZED');
    expect(op!.profile).toBe('bluecash-app');
    expect(op!.fiscal_reference).toMatch(/^E2E-bluecash-app-/);

    // Card terminal fields
    expect(op!.rrn).toBe('123456789012');
    expect(op!.authorization_code).toBe('E2E001');
    expect(typeof op!.payment_reference).toBe('string');
    expect(op!.payment_reference).toMatch(/^PAY-/);
  });

  test('split payment: card portion sent to bluecash-app, intent has both payments', async ({ page }) => {
    let capturedIntent: Record<string, unknown> | null = null;

    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      adapterBase: BLUECASH_APP_BASE,
    });

    await page.route(`${BLUECASH_APP_BASE}/intents`, async (route) => {
      capturedIntent = JSON.parse(route.request().postData() || '{}');
      await route.continue();
    });

    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    // Кафе = 2.50; enter card amount = 1.00 → CARD 1.00 + CASH 1.50
    await page.getByRole('button', { name: /Кафе/ }).click();
    await expect(page.locator('.total')).toContainText('2.50');

    await page.locator('input[inputMode="decimal"]').fill('1.00');
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    // Intent received at bluecash-app includes both CARD and CASH payments
    expect(capturedIntent).not.toBeNull();
    const payments = capturedIntent!['payments'] as Array<Record<string, unknown>>;
    expect(Array.isArray(payments)).toBe(true);
    expect(payments.length).toBe(2);

    const cardPay = payments.find((p) => p['type'] === 'CARD');
    const cashPay = payments.find((p) => p['type'] === 'CASH');
    expect(cardPay).toBeDefined();
    expect(cashPay).toBeDefined();
    expect(cardPay!['amount']).toBe('1.00');
    expect(cashPay!['amount']).toBe('1.50');
  });

  test('card sale idempotency: same operation returned on duplicate Idempotency-Key', async ({ page }) => {
    const intentCalls: Array<string> = [];

    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      adapterBase: BLUECASH_APP_BASE,
    });

    await page.route(`${BLUECASH_APP_ORIGIN}/**`, async (route) => {
      const url = route.request().url();
      if (url.includes('/intents')) {
        const key = route.request().headers()['idempotency-key'] ?? '';
        intentCalls.push(key);
      }
      await route.continue();
    });

    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.locator('input[inputMode="decimal"]').fill('2.50');
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    // Only one intent submission per checkout (no duplicates)
    expect(intentCalls.length).toBe(1);
    expect(intentCalls[0].length).toBe(36); // UUID
  });

  test('fiscal_reference from bluecash-app shown in settings result panel', async ({ page }) => {
    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      adapterBase: BLUECASH_APP_BASE,
    });
    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.locator('input[inputMode="decimal"]').fill('2.50');
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    await page.getByRole('button', { name: 'Настройки' }).click();
    await expect(page.locator('pre')).toContainText('E2E-bluecash-app-', { timeout: 5_000 });
    await expect(page.locator('pre')).toContainText('FISCALIZED');
  });
});
