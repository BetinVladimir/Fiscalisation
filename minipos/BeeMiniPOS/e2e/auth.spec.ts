import { test, expect } from '@playwright/test';
import { setupMiniposRoutes, setupFiscalRoutes, TEST_JWT, TEST_REFRESH } from './helpers';

test.describe('Email auth flow', () => {
  test('email → OTP code → authenticated (no onboarding)', async ({ page }) => {
    await setupMiniposRoutes(page);
    await setupFiscalRoutes(page);
    await page.goto('/');

    await page.getByTestId('operator-login').waitFor({ timeout: 5_000 });

    await page.locator('[placeholder="email@example.com"]').fill('operator@example.com');
    await page.getByTestId('operator-login-start').click();

    await page.locator('[placeholder="000000"]').fill('123456');
    await page.locator('[role="button"]').filter({ hasText: /Продължи|Continue/ }).click();

    // After auth → startup → product catalog visible
    await page.getByTestId('status-transport').filter({ hasText: /Готово|Отворена/ }).waitFor({ timeout: 15_000 });
    await expect(page.getByTestId('product-coffee')).toBeVisible();
  });

  test('email → OTP → onboarding → authenticated', async ({ page }) => {
    await page.route('**/auth/verify-code', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ access_token: '', refresh_token: '', expires_in: 0, onboarding_required: true, onboarding_token: 'onboard-tok' }),
      }),
    );
    await page.route('**/auth/onboarding', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600 }),
      }),
    );
    await setupMiniposRoutes(page);
    await setupFiscalRoutes(page);
    await page.goto('/');

    await page.locator('[placeholder="email@example.com"]').fill('new@example.com');
    await page.getByTestId('operator-login-start').click();
    await page.locator('[placeholder="000000"]').fill('999999');
    await page.locator('[role="button"]').filter({ hasText: /Продължи|Continue/ }).click();

    // Onboarding screen
    await page.locator('[placeholder*="компани"], [placeholder*="Company"]').first().fill('Тест ЕООД');
    await page.locator('[placeholder*="дрес"], [placeholder*="ddress"]').first().fill('ул. Тест 1');
    await page.locator('[placeholder*="данъч"], [placeholder*="ax"]').first().fill('BG123456789');
    await page.locator('[placeholder*="Вашите имена"], [placeholder*="full name"]').first().fill('Иван Иванов');
    await page.locator('[role="button"]').filter({ hasText: /Създай|Create/ }).click();

    await page.getByTestId('status-transport').filter({ hasText: /Готово/ }).waitFor({ timeout: 15_000 });
    await expect(page.getByTestId('product-coffee')).toBeVisible();
  });

  test('token auto-refresh: tokens stored in AsyncStorage are used on reload', async ({ page }) => {
    await setupMiniposRoutes(page);
    await setupFiscalRoutes(page);

    // Pre-set tokens so refresh runs on mount
    await page.addInitScript(([tok, ref]) => {
      localStorage.setItem('minipos-access-token', tok);
      localStorage.setItem('minipos-refresh-token', ref);
    }, [TEST_JWT, TEST_REFRESH] as const);

    await page.goto('/');

    // App should auto-auth via refresh and show the POS
    await page.getByTestId('status-transport').filter({ hasText: /Готово|Отворена/ }).waitFor({ timeout: 15_000 });
    await expect(page.getByTestId('product-coffee')).toBeVisible();
  });

  test('refresh token failure → shows login screen', async ({ page }) => {
    await page.route('**/auth/refresh', (route) => route.fulfill({ status: 401, body: JSON.stringify({ code: 'INVALID_REFRESH_TOKEN' }), contentType: 'application/json' }));
    await page.addInitScript(([tok, ref]) => {
      localStorage.setItem('minipos-access-token', tok);
      localStorage.setItem('minipos-refresh-token', ref);
    }, [TEST_JWT, 'expired-refresh-token'] as const);

    await page.goto('/');

    await page.getByTestId('operator-login').waitFor({ timeout: 8_000 });
  });

  test('invalid OTP → shows error text', async ({ page }) => {
    await page.route('**/auth/verify-code', (route) =>
      route.fulfill({ status: 401, body: 'Неверен код', contentType: 'text/plain' }),
    );
    await page.goto('/');

    await page.locator('[placeholder="email@example.com"]').fill('operator@example.com');
    await page.getByTestId('operator-login-start').click();
    await page.locator('[placeholder="000000"]').fill('000000');
    await page.locator('[role="button"]').filter({ hasText: /Продължи/ }).click();

    await expect(page.locator('[style*="loginError"], [role="text"]').filter({ hasText: /Неверен|код|error/i })).toBeVisible({ timeout: 5_000 });
  });
});
