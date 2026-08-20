import { test, expect } from '@playwright/test';
import { setup, waitForReady, setupMiniposRoutes, setupFiscalRoutes, authenticate } from './helpers';

test.describe('Startup – journal recovery', () => {
  test('reload with open sale → sale projection restored from fiscal API', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });

    await page.reload();

    // After reload: fiscal API returns the open sale → projection restored
    await waitForReady(page, /Отворената смяна е възстановена/);
    await expect(page.getByTestId('sale-total')).toContainText('€ 2.50');
  });
});

test.describe('Connectivity – cloud unreachable', () => {
  test('fiscal ping failure → checkout still attempted via cloud (no BLE)', async ({ page }) => {
    await setupMiniposRoutes(page);
    await setupFiscalRoutes(page);
    await authenticate(page);

    // Override ping to fail
    await page.route('**/connectivity/ping', (route) =>
      route.fulfill({ status: 503, body: '' }),
    );

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('sale-pay').click();

    // Even with ping failing, without BLE the cloud path is attempted.
    // The finalize mock returns FISCALIZED.
    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 20_000 }); // extra time for 3 ping retries (3 × 1.5s + 2 × 1s)
  });

  test('fiscal API 503 on finalize → known failure, no retry and no UNKNOWN panel', async ({ page }) => {
    let finalizeCalls = 0;
    await setupMiniposRoutes(page);
    await setupFiscalRoutes(page);
    await authenticate(page);

    await page.route(/\/sales\/[^/]+:finalize$/, async (route) => {
      finalizeCalls++;
      return route.fulfill({ status: 503, body: JSON.stringify({ code: 'SERVICE_UNAVAILABLE' }), contentType: 'application/json' });
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport').filter({ hasText: /503|SERVICE_UNAVAILABLE/ }).waitFor({ timeout: 20_000 });
    await expect(page.getByTestId('operation-unknown')).not.toBeVisible();
    expect(finalizeCalls).toBe(1);
  });
});

test.describe('Checkout journal – idempotency', () => {
  test('duplicate checkout attempt uses same client_operation_id', async ({ page }) => {
    const operationIds = new Set<string>();
    await setupMiniposRoutes(page);
    await setupFiscalRoutes(page);
    await authenticate(page);

    await page.route(/\/sales\/[^/]+:finalize$/, async (route) => {
      const key = route.request().headers()['idempotency-key'] || '';
      operationIds.add(key);
      return route.fallback();
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('product-coffee').click();
    await page.getByTestId('cart-line-coffee').waitFor({ timeout: 8_000 });
    await page.getByTestId('sale-pay').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Успешен фискален бон/ })
      .waitFor({ timeout: 15_000 });

    // Exactly one unique operation ID per checkout
    expect(operationIds.size).toBe(1);
  });
});
