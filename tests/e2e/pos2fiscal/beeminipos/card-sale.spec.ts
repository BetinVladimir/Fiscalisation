/**
 * BeeMiniPOS – CARD sale, full flow audit
 *
 * The UI "Карта" button triggers a checkout plan with payment type CARD and
 * terminal_policy AUTO_IF_AVAILABLE. The finalize request carries this through to
 * fiscal-backend, which routes it to the physical card terminal (bluecash-app/BlueCash-50).
 *
 * POS terminal control: POST /sales/{id}:finalize body verified for CARD type + policy.
 * Fiscal device control: fiscal_reference from device round-trip verified in UI.
 */

import { test, expect } from '@playwright/test';
import { setupBeeMiniposRoutes, authenticate, waitForBeeMiniposReady } from '../helpers';

test.describe('BeeMiniPOS – CARD sale', () => {
  test('CARD checkout → finalize with CARD payment + AUTO_IF_AVAILABLE policy', async ({ page }) => {
    let finalizeBody: Record<string, unknown> | null = null;

    await setupBeeMiniposRoutes(page, {
      onFinalize(body) { finalizeBody = body; },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });

    // CARD payment button
    await page.getByTestId('payment-card').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // ── POS terminal audit ────────────────────────────────────────────────────
    expect(finalizeBody).not.toBeNull();

    const payments = finalizeBody!['payments'] as Array<Record<string, unknown>>;
    expect(Array.isArray(payments)).toBe(true);
    expect(payments.length).toBe(1);

    const cardPay = payments[0];
    expect(cardPay['type']).toBe('CARD');
    expect(cardPay['amount']).toEqual({ amount: '2.50', currency: 'EUR' });

    // terminal_policy must be AUTO_IF_AVAILABLE for CARD payments
    expect(cardPay['terminal_policy']).toBe('AUTO_IF_AVAILABLE');

    // expected_total
    const total = finalizeBody!['expected_total'] as Record<string, unknown>;
    expect(total['amount']).toBe('2.50');
    expect(total['currency']).toBe('EUR');
  });

  test('client_operation_id is UUID and matches Idempotency-Key header', async ({ page }) => {
    let finalizeBody: Record<string, unknown> | null = null;
    let idempKey: string | null = null;

    await setupBeeMiniposRoutes(page, {
      onFinalize(body, headers) {
        finalizeBody = body;
        idempKey = headers['idempotency-key'] ?? null;
      },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    const clientOpId = finalizeBody!['client_operation_id'] as string;
    expect(typeof clientOpId).toBe('string');
    expect(clientOpId.length).toBe(36); // UUID v4

    // Header must match body field for idempotent replay detection
    expect(idempKey).toBe(clientOpId);
  });

  test('CARD sale fiscal_reference propagated to UI', async ({ page }) => {
    const FISCAL_REF = 'FD-P2F-CARD-777';

    await setupBeeMiniposRoutes(page, {
      finalizeResponse: {
        operation_id: 'op-bmp-card-777',
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
    await page.getByTestId('payment-card').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    await expect(page.getByText(FISCAL_REF)).toBeVisible({ timeout: 5_000 });
  });

  test('CARD sale UNKNOWN result → operation-unknown panel, cart not cleared', async ({ page }) => {
    await setupBeeMiniposRoutes(page, {
      finalizeState: 'UNKNOWN',
      finalizeResponse: {
        operation_id: 'op-bmp-card-unknown',
        type: 'FISCAL_SALE',
        state: 'UNKNOWN',
        fiscal_reference: null,
        allowed_actions: ['RECONCILE', 'READ'],
      },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    // UNKNOWN result shows the pending panel
    await page.getByTestId('operation-unknown').waitFor({ timeout: 20_000 });
    // Cart is NOT cleared — awaiting reconciliation
    await expect(page.getByTestId('sale-total')).toContainText('€ 2.50');
  });

  test('CARD sale: no duplicate finalize calls per checkout', async ({ page }) => {
    let finalizeCalls = 0;

    await setupBeeMiniposRoutes(page, {
      onFinalize() { finalizeCalls++; },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // Must be exactly 1 — no duplicate submissions
    expect(finalizeCalls).toBe(1);
  });
});
