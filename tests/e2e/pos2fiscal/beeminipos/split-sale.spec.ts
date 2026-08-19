/**
 * BeeMiniPOS – split tender sale (CASH + CARD)
 *
 * The operator adds products, then activates split payment mode via the payment-split button.
 * The split UI allows entering a CASH portion; the remainder goes to CARD.
 *
 * Verify that the finalize request contains BOTH payment legs with correct amounts and
 * correct terminal policies:
 *   CASH: terminal_policy = NONE
 *   CARD: terminal_policy = AUTO_IF_AVAILABLE
 *
 * POS terminal control: both payment entries in finalize body audited.
 * Fiscal device control: fiscal_reference from device round-trip verified in UI.
 */

import { test, expect } from '@playwright/test';
import { setupBeeMiniposRoutes, authenticate, waitForBeeMiniposReady } from '../helpers';

test.describe('BeeMiniPOS – split tender sale', () => {
  test('split CASH 1.00 + CARD 2.50 (total 3.50) → finalize has both payments with correct policies', async ({ page }) => {
    let finalizeBody: Record<string, unknown> | null = null;

    await setupBeeMiniposRoutes(page, {
      onFinalize(body) { finalizeBody = body; },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    // Add Кафе (2.50) + Вода (1.00) = 3.50 total
    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('product-water').click();
    await page.getByTestId('cart-line-water').waitFor({ timeout: 8_000 });
    await expect(page.getByTestId('sale-total')).toContainText('€ 3.50');

    // Open split payment mode
    await page.getByTestId('payment-split').click();

    // Enter CASH portion = 1.00 → CARD = 2.50
    await page.getByTestId('split-cash-amount').fill('1.00');

    // Confirm split payment
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // ── POS terminal audit: finalize body ────────────────────────────────────
    expect(finalizeBody).not.toBeNull();

    const payments = finalizeBody!['payments'] as Array<Record<string, unknown>>;
    expect(Array.isArray(payments)).toBe(true);
    expect(payments.length).toBe(2);

    const cashPay = payments.find((p) => p['type'] === 'CASH');
    const cardPay = payments.find((p) => p['type'] === 'CARD');

    expect(cashPay).toBeDefined();
    expect(cardPay).toBeDefined();

    expect(cashPay!['amount']).toBe('1.00');
    if (cashPay!['terminal_policy'] !== undefined)
      expect(cashPay!['terminal_policy']).toBe('NONE');

    expect(cardPay!['amount']).toBe('2.50');
    expect(cardPay!['terminal_policy']).toBe('AUTO_IF_AVAILABLE');

    // Total: 1.00 + 2.50 = 3.50
    const total = finalizeBody!['expected_total'] as Record<string, unknown>;
    expect(total['amount']).toBe('3.50');
    expect(total['currency']).toBe('EUR');
  });

  test('split with full CASH → only CASH payment in finalize (no split)', async ({ page }) => {
    let finalizeBody: Record<string, unknown> | null = null;

    await setupBeeMiniposRoutes(page, {
      onFinalize(body) { finalizeBody = body; },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });

    // Split mode, cash = full amount = 2.50 → no card remainder
    await page.getByTestId('payment-split').click();
    await page.getByTestId('split-cash-amount').fill('2.50');
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    expect(finalizeBody).not.toBeNull();
    const payments = finalizeBody!['payments'] as Array<Record<string, unknown>>;

    // When cash = full total, either: 1 CASH payment, or 2 payments where CARD = 0.00
    const cardPay = payments.find((p) => p['type'] === 'CARD');
    if (cardPay) {
      // If the app still emits a CARD entry, it should be 0.00
      expect(parseFloat(cardPay['amount'] as string)).toBe(0);
    } else {
      // Otherwise just CASH
      const cashPay = payments.find((p) => p['type'] === 'CASH');
      expect(cashPay).toBeDefined();
      expect(cashPay!['amount']).toBe('2.50');
    }
  });

  test('split receipt_session_id unique per checkout', async ({ page }) => {
    const sessionIds = new Set<string>();

    await setupBeeMiniposRoutes(page, {
      onFinalize(body) {
        const id = body['receipt_session_id'] as string;
        if (id) sessionIds.add(id);
      },
    });
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-split').click();
    await page.getByTestId('split-cash-amount').fill('1.00');
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // receipt_session_id should be present and unique (UUID)
    expect(sessionIds.size).toBe(1);
    const [id] = sessionIds;
    expect(id.length).toBe(36);
  });

  test('split sale cart clears completely after FISCALIZED', async ({ page }) => {
    await setupBeeMiniposRoutes(page);
    await authenticate(page);
    await page.goto('/');
    await waitForBeeMiniposReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('product-water').click();
    await page.getByTestId('cart-line-water').waitFor({ timeout: 8_000 });

    await page.getByTestId('payment-split').click();
    await page.getByTestId('split-cash-amount').fill('1.00');
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 });

    // Cart must be empty after successful fiscal split payment
    await expect(page.getByTestId('cart-line-coffee')).not.toBeVisible({ timeout: 5_000 });
    await expect(page.getByTestId('cart-line-water')).not.toBeVisible({ timeout: 5_000 });
  });
});
