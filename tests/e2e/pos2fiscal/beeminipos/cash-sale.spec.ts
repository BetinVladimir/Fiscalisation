/**
 * BeeMiniPOS – CASH sale, full flow audit
 *
 * Flow: UI (add product) → fiscal-backend (POST /sales:open-with-line) →
 *       fiscal-backend (POST /sales/{id}:finalize) → Datecs/Daisy (edge-agent).
 *
 * In BeeMiniPOS the browser does NOT call the edge-agent directly — fiscal-backend
 * is the Datecs boundary from the browser's perspective. The finalize request carries
 * the checkout plan (payments, expected_total, client_operation_id).
 *
 * POS terminal control: POST /sales:open-with-line body (product line added to sale)
 * Fiscal device control: POST /sales/{id}:finalize body (payments, amount, idempotency)
 */

import { test, expect } from '@playwright/test';
import { setupBeeMiniposRoutes, authenticate, waitForBeeMiniposReady } from '../helpers';

test.describe('BeeMiniPOS – CASH sale', () => {
  test('add product → finalize → FISCALIZED shown in UI', async ({ page }) => {
    await setupBeeMiniposRoutes(page);
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await expect(page.getByTestId('sale-total')).toContainText('€ 2.50');

    // CASH checkout (default sale-pay)
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });
  });

  test('finalize request body: CASH payment, correct amount, terminal_policy NONE', async ({ page }) => {
    let finalizeBody: Record<string, unknown> | null = null;
    let finalizeHeaders: Record<string, string> = {};

    await setupBeeMiniposRoutes(page, {
      onFinalize(body, headers) {
        finalizeBody = body;
        finalizeHeaders = headers;
      },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // ── POS terminal audit: finalize request ─────────────────────────────────
    expect(finalizeBody).not.toBeNull();

    // client_operation_id (idempotency, UUID)
    expect(typeof finalizeBody!['client_operation_id']).toBe('string');
    expect((finalizeBody!['client_operation_id'] as string).length).toBe(36);

    // Idempotency-Key header matches client_operation_id
    const idempKey = finalizeHeaders['idempotency-key'];
    expect(idempKey).toBe(finalizeBody!['client_operation_id']);

    // payments: single CASH payment for €2.50
    const payments = finalizeBody!['payments'] as Array<Record<string, unknown>>;
    expect(Array.isArray(payments)).toBe(true);
    expect(payments.length).toBe(1);

    const cashPay = payments[0];
    expect(cashPay['type']).toBe('CASH');
    expect(cashPay['amount']).toEqual({ amount: '2.50', currency: 'EUR' });

    // CASH does not use card terminal → no terminal_policy or NONE
    if (cashPay['terminal_policy'] !== undefined)
      expect(cashPay['terminal_policy']).toBe('NONE');

    // expected_total matches cart total
    const total = finalizeBody!['expected_total'] as Record<string, unknown>;
    expect(total['amount']).toBe('2.50');
    expect(total['currency']).toBe('EUR');
  });

  test('open-with-line request: product line forwarded to fiscal-backend correctly', async ({ page }) => {
    let openWithLineBody: Record<string, unknown> | null = null;

    await setupBeeMiniposRoutes(page, {
      onOpenWithLine(body) {
        openWithLineBody = body;
      },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // ── POS terminal audit: open-with-line request ────────────────────────────
    expect(openWithLineBody).not.toBeNull();

    // client_sale_surrogate_id (UUID)
    expect(typeof openWithLineBody!['client_sale_surrogate_id']).toBe('string');

    // line contains product information
    const line = openWithLineBody!['line'] as Record<string, unknown>;
    expect(line).toBeDefined();
    expect(line['tax_group']).toBe('B');

    const unitPrice = line['unit_price'] as Record<string, unknown>;
    expect(unitPrice['amount']).toBe('2.50');
    expect(unitPrice['currency']).toBe('EUR');
  });

  test('fiscal_reference from finalize response shown in UI', async ({ page }) => {
    const FISCAL_REF = 'FD-P2F-CASH-999';

    await setupBeeMiniposRoutes(page, {
      finalizeResponse: {
        operation_id: 'op-bmp-cash-999',
        type: 'FISCAL_SALE',
        state: 'FISCALIZED',
        fiscal_reference: FISCAL_REF,
        allowed_actions: [],
      },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('sale-pay').click();

    // The fiscal_reference should appear in the success status text
    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // Also present in the pending order result shown in the UI
    await expect(page.getByText(FISCAL_REF)).toBeVisible({ timeout: 5_000 });
  });
});
