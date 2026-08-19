import { test, expect } from '@playwright/test';
import { setupMiniposRoutes, TEST_JWT, TEST_REFRESH } from './helpers';

test.describe('Auth flow', () => {
  test('email → OTP → authenticated (no onboarding)', async ({ page }) => {
    await setupMiniposRoutes(page, {
      // verifyCode returns tokens without onboarding
      verifyCode: { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600, onboarding_required: false },
    });
    await page.goto('/');

    await expect(page.getByText('Вход в BeeMiniPOS')).toBeVisible();

    await page.getByLabel('Имейл').fill('operator@example.com');
    await page.getByRole('button', { name: 'Изпрати код' }).click();

    await expect(page.getByText('operator@example.com')).toBeVisible();

    await page.getByLabel('Еднократен код').fill('123456');
    await page.getByRole('button', { name: 'Продължи' }).click();

    // After auth, the POS main view should appear
    await expect(page.getByText('Кафе')).toBeVisible({ timeout: 8_000 });
  });

  test('email → OTP → onboarding → authenticated', async ({ page }) => {
    await setupMiniposRoutes(page, {
      verifyCode: {
        access_token: '',
        refresh_token: '',
        expires_in: 0,
        onboarding_required: true,
        onboarding_token: 'onboard-tok-001',
      },
    });
    await page.goto('/');

    await page.getByLabel('Имейл').fill('new@example.com');
    await page.getByRole('button', { name: 'Изпрати код' }).click();
    await page.getByLabel('Еднократен код').fill('654321');
    await page.getByRole('button', { name: 'Продължи' }).click();

    await expect(page.getByText('Данни за компанията')).toBeVisible();

    await page.getByLabel('Наименование').fill('Моята Компания');
    await page.getByLabel('Адрес').fill('ул. Тест 1, София');
    await page.getByLabel('Данъчен идентификатор').fill('BG123456789');
    await page.getByLabel('Вашите имена').fill('Иван Иванов');
    await page.getByRole('button', { name: 'Създай компания' }).click();

    await expect(page.getByText('Кафе')).toBeVisible({ timeout: 8_000 });
  });

  test('language switch: Bulgarian → Russian → English', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('Вход в BeeMiniPOS')).toBeVisible();

    // Switch to Russian
    await page.getByLabel('Language').selectOption('ru');
    await expect(page.getByRole('button', { name: 'Отправить код' })).toBeVisible();

    // Switch to English
    await page.getByLabel('Language').selectOption('en');
    await expect(page.getByText('Sign in to BeeMiniPOS')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Send code' })).toBeVisible();

    // Back to Bulgarian
    await page.getByLabel('Language').selectOption('bg');
    await expect(page.getByRole('button', { name: 'Изпрати код' })).toBeVisible();
  });

  test('wrong OTP shows error', async ({ page }) => {
    await setupMiniposRoutes(page, {
      verifyCode: 401,
    });
    await page.goto('/');

    await page.getByLabel('Имейл').fill('operator@example.com');
    await page.getByRole('button', { name: 'Изпрати код' }).click();
    await page.getByLabel('Еднократен код').fill('000000');
    await page.getByRole('button', { name: 'Продължи' }).click();

    await expect(page.locator('.error')).toBeVisible();
  });

  test('request-code failure shows error', async ({ page }) => {
    await setupMiniposRoutes(page, { requestCode: 500 });
    await page.goto('/');

    await page.getByLabel('Имейл').fill('bad@example.com');
    await page.getByRole('button', { name: 'Изпрати код' }).click();

    await expect(page.locator('.error')).toBeVisible();
  });

  test('continue button disabled until 6-digit code entered', async ({ page }) => {
    await setupMiniposRoutes(page);
    await page.goto('/');

    await page.getByLabel('Имейл').fill('operator@example.com');
    await page.getByRole('button', { name: 'Изпрати код' }).click();

    const continueBtn = page.getByRole('button', { name: 'Продължи' });
    await expect(continueBtn).toBeDisabled();

    await page.getByLabel('Еднократен код').fill('12345');
    await expect(continueBtn).toBeDisabled();

    await page.getByLabel('Еднократен код').fill('123456');
    await expect(continueBtn).toBeEnabled();
  });
});
