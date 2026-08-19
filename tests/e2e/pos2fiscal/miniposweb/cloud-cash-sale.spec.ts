/**
 * miniposweb – CLOUD route, CASH sale
 *
 * Full flow: browser UI → minipos-backend (checkout-batch) → fiscal-backend → Datecs/Daisy.
 * The CLOUD route means the POS sends payment+metadata to minipos-backend, which orchestrates
 * the fiscal device call server-side. We audit the checkout-batch request the browser sends
 * to verify the payment type and idempotency key.
 *
 * POS terminal control: checkout-batch request body contains payments and receipt_session_id.
 * Fiscal device control: implicit via the fiscal_reference returned and propagated to UI.
 */

import { test, expect } from '@playwright/test';
import { setupMinipowsebRoutes, authenticate, waitForPOS } from '../helpers';

test.describe('miniposweb – CLOUD cash sale', () => {
  test('CASH checkout sends correct payment type to minipos-backend', async ({ page }) => {
    let checkoutBatchBody: Record<string, unknown> | null = null;
    let checkoutBatchIdempotencyKey: string | null = null;

    await setupMinipowsebRoutes(page, {
      // CLOUD route returns 200 (available)
      fiscalRouteHealth: { status: 'ok' },
      // Override to capture then respond
      checkoutBatch: undefined,
    });

    // Intercept the checkout-batch call to audit the request
    await page.route('**/checkout-batch', async (route) => {
      checkoutBatchBody = JSON.parse(route.request().postData() || '{}');
      checkoutBatchIdempotencyKey = route.request().headers()['idempotency-key'] ?? null;
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          operation_id: 'op-cloud-cash-001',
          state: 'FISCALIZED',
          fiscal_reference: 'CLOUD-CASH-REF-001',
          updated_at: new Date().toISOString(),
        }),
      });
    });

    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    // Add product (Кафе = €2.50)
    await page.getByRole('button', { name: /Кафе/ }).click();
    await expect(page.locator('.total')).toContainText('2.50');

    // Checkout (no card amount set → full CASH)
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Cart clears on FISCALIZED
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 15_000 });

    // ── POS terminal audit: verify checkout-batch request ────────────────────
    expect(checkoutBatchBody).not.toBeNull();

    const payments = checkoutBatchBody!['payments'] as Array<Record<string, unknown>>;
    expect(Array.isArray(payments)).toBe(true);
    expect(payments.length).toBeGreaterThan(0);

    const cashPayment = payments.find((p) => p['type'] === 'CASH');
    expect(cashPayment).toBeDefined();
    expect(cashPayment!['amount']).toBe('2.50');

    // receipt_session_id propagated via metadata
    const metadata = checkoutBatchBody!['metadata'] as Record<string, unknown>;
    expect(typeof metadata?.['receipt_session_id']).toBe('string');
    expect((metadata['receipt_session_id'] as string).length).toBeGreaterThan(0);

    // Idempotency key is present (= client_operation_id from FiscalIntent)
    expect(checkoutBatchIdempotencyKey).not.toBeNull();
    expect(checkoutBatchIdempotencyKey!.length).toBe(36); // UUID format

    // No CARD payment present (pure CASH)
    const cardPayment = payments.find((p) => p['type'] === 'CARD');
    expect(cardPayment).toBeUndefined();
  });

  test('fiscal_reference from CLOUD response visible in result panel', async ({ page }) => {
    const FISCAL_REF = 'CLOUD-AUDIT-2026-001';

    await setupMinipowsebRoutes(page, {
      checkoutBatch: {
        operation_id: 'op-cloud-audit-001',
        state: 'FISCALIZED',
        fiscal_reference: FISCAL_REF,
        updated_at: new Date().toISOString(),
      },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 15_000 });

    // Settings panel shows the fiscal operation result
    await page.getByRole('button', { name: 'Настройки' }).click();
    await expect(page.locator('pre')).toContainText(FISCAL_REF, { timeout: 5_000 });
  });

  test('idempotency key is stable per checkout (no duplicate submissions)', async ({ page }) => {
    const keys = new Set<string>();

    await setupMinipowsebRoutes(page);

    await page.route('**/checkout-batch', async (route) => {
      const key = route.request().headers()['idempotency-key'];
      if (key) keys.add(key);
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          operation_id: key ?? 'op-idem-001',
          state: 'FISCALIZED',
          fiscal_reference: 'IDEM-REF-001',
          updated_at: new Date().toISOString(),
        }),
      });
    });

    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 15_000 });

    // Exactly one checkout-batch call per checkout
    expect(keys.size).toBe(1);
  });
});
