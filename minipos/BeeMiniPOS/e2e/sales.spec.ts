import { test, expect } from '@playwright/test';
import { setup, waitForReady, REGISTER_ID } from './helpers';

test.describe('Sales – cash checkout', () => {
  test('add product → cash checkout → FISCALIZED status', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();

    await expect(page.getByTestId('sale-total')).toContainText('€ 2.50', { timeout: 8_000 });
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 15_000 });
    await expect(page.getByTestId('sale-total')).toHaveText('€ 0.00');
  });

  test('add same product twice → quantity 2 in cart', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    // Wait for first line to appear
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('product-coffee').click();

    await expect(page.getByTestId('sale-total')).toContainText('€ 5.00', { timeout: 8_000 });
  });

  test('increase/decrease quantity buttons', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });

    await page.getByTestId('quantity-inc-coffee').click();
    await expect(page.getByTestId('sale-total')).toContainText('€ 5.00', { timeout: 8_000 });

    await page.getByTestId('quantity-dec-coffee').click();
    await expect(page.getByTestId('sale-total')).toContainText('€ 2.50', { timeout: 5_000 });
  });

  test('remove item via decrement to zero → line removed', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('quantity-dec-coffee').click();

    await expect(page.getByTestId('cart-line-coffee')).not.toBeVisible({ timeout: 5_000 });
    await expect(page.getByTestId('sale-total')).toContainText('€ 0.00');
  });

  test('barcode/SKU search adds product', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-search').fill('380000000001');
    await page.getByTestId('product-search').press('Enter');

    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    // Search input clears after successful lookup
    await expect(page.getByTestId('product-search')).toHaveValue('');
  });

  test('unknown barcode shows error in status', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-search').fill('999999999999');
    await page.getByTestId('product-search').press('Enter');

    await page.getByTestId('status-transport')
      .filter({ hasText: /Няма продукт/ })
      .waitFor({ timeout: 5_000 });
  });
});

test.describe('Sales – card checkout', () => {
  test('card checkout → FISCALIZED', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 15_000 });
  });

  test('card checkout FAILED → no UNKNOWN panel', async ({ page }) => {
    await setup(page, { fiscal: { finalizeState: 'FAILED' } });
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /PAYMENT_REJECTED/ })
      .waitFor({ timeout: 15_000 });
    // FAILED is known → no ambiguous UNKNOWN panel
    await expect(page.getByTestId('operation-unknown')).not.toBeVisible();
  });
});

test.describe('Sales – split tender', () => {
  test('split payment cash+card → FISCALIZED', async ({ page }) => {
    const seenFinalize: unknown[] = [];
    await setup(page);

    await page.route('**/sales:open-with-line', async (route) => {
      await route.fallback();
    });
    await page.route(/\/sales\/[^/]+:finalize$/, async (route) => {
      const body = JSON.parse(route.request().postData() || '{}');
      seenFinalize.push(body);
      await route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });

    await page.getByTestId('split-cash-amount').fill('1,00');
    await page.getByTestId('payment-split').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 15_000 });

    // Verify payment split structure
    const plan = seenFinalize[0] as any;
    expect(plan?.payments?.length).toBe(2);
    const cash = plan?.payments?.find((p: any) => p.type === 'CASH');
    const card = plan?.payments?.find((p: any) => p.type === 'CARD');
    expect(cash?.amount.amount).toBe('1.00');
    expect(card?.amount.amount).toBe('1.50');
    expect(card?.terminal_policy).toBe('AUTO_IF_AVAILABLE');
  });

  test('split: cash >= total → rejects with status message', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });

    // Enter cash amount equal to total → invalid split
    await page.getByTestId('split-cash-amount').fill('2.50');
    await page.getByTestId('payment-split').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Разделено плащане: въведете/ })
      .waitFor({ timeout: 5_000 });
  });
});

test.describe('Sales – UNKNOWN reconciliation', () => {
  test('UNKNOWN result → operation-unknown panel shown, reconcile resolves it', async ({ page }) => {
    await setup(page, { fiscal: { finalizeState: 'UNKNOWN' } });
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    await page.getByTestId('operation-unknown').waitFor({ timeout: 15_000 });
    await expect(page.getByTestId('reconcile-start')).toBeVisible();

    // Reconcile resolves to FISCALIZED
    await page.getByTestId('reconcile-start').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /потвърден/ })
      .waitFor({ timeout: 10_000 });
    await expect(page.getByTestId('operation-unknown')).not.toBeVisible();
  });

  test('reconcile does not re-submit the payment', async ({ page }) => {
    let finalizeCalls = 0;
    await setup(page, { fiscal: { finalizeState: 'UNKNOWN' } });

    await page.route(/\/sales\/[^/]+:finalize$/, async (route) => {
      finalizeCalls++;
      await route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    await page.getByTestId('operation-unknown').waitFor({ timeout: 15_000 });
    const callsBeforeReconcile = finalizeCalls;
    await page.getByTestId('reconcile-start').click();
    await page.getByTestId('status-transport').filter({ hasText: /потвърден/ }).waitFor({ timeout: 10_000 });

    expect(finalizeCalls).toBe(callsBeforeReconcile);
  });
});

test.describe('Sales – finalize 5xx → ambiguous UNKNOWN panel', () => {
  test('network error on finalize → sets pendingOrder with UNKNOWN state', async ({ page }) => {
    await setup(page);
    await page.route(/\/sales\/[^/]+:finalize$/, (route) => route.abort('failed'));
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('payment-card').click();

    // Network error on finalize → UNKNOWN panel
    await page.getByTestId('operation-unknown').waitFor({ timeout: 15_000 });
    await expect(page.getByTestId('status-transport')).toContainText('не повтаряйте');
  });
});
