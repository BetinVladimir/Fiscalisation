import { test, expect } from '@playwright/test';
import { setup, SESSION, CONFIG } from './helpers';

test.describe('Shift lifecycle', () => {
  test('no shift → shows "Отвори смяна" button', async ({ page }) => {
    await setup(page, { minipos: { shifts: { items: [] } } });
    await page.goto('/');

    // Session and config load OK but no shift
    await expect(page.getByRole('button', { name: 'Отвори смяна' })).toBeVisible({ timeout: 8_000 });
  });

  test('open shift → shift appears → products visible', async ({ page }) => {
    let shiftOpen = false;
    await setup(page, {
      minipos: {
        shifts: { items: [] },
        openShift: {
          id: 'shift-new-001',
          register_id: 'reg-001',
          employee_id: 'emp-001',
          state: 'OPEN',
          version: 1,
        },
      },
    });

    // After openShift, the subsequent shifts GET should return the new shift
    await page.route('**/public/v1/minipos/shifts**', async (route) => {
      if (route.request().method() === 'POST') {
        shiftOpen = true;
        return route.fulfill({
          status: 200,
          body: JSON.stringify({ id: 'shift-new-001', register_id: 'reg-001', employee_id: 'emp-001', state: 'OPEN', version: 1 }),
          contentType: 'application/json',
        });
      }
      return route.fulfill({
        status: 200,
        body: JSON.stringify({ items: shiftOpen ? [{ id: 'shift-new-001', register_id: 'reg-001', employee_id: 'emp-001', state: 'OPEN', version: 1 }] : [] }),
        contentType: 'application/json',
      });
    });

    await page.goto('/');

    await page.getByRole('button', { name: 'Отвори смяна' }).click();

    await expect(page.getByText('Кафе')).toBeVisible({ timeout: 8_000 });
    expect(shiftOpen).toBe(true);
  });

  test('no configuration → open shift button disabled', async ({ page }) => {
    await setup(page, {
      minipos: {
        shifts: { items: [] },
        configuration: 404,
      },
    });
    await page.goto('/');

    const openBtn = page.getByRole('button', { name: 'Отвори смяна' });
    await expect(openBtn).toBeVisible({ timeout: 8_000 });
    await expect(openBtn).toBeDisabled();
  });

  test('settings panel shows adapter configuration', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await expect(page.getByText('Кафе')).toBeVisible({ timeout: 8_000 });

    await page.getByRole('button', { name: 'Настройки' }).click();

    await expect(page.getByText('adapter-001')).toBeVisible();
    await expect(page.getByText('reg-001')).toBeVisible();
    await expect(page.locator('text=Generation: 1')).toBeVisible();
  });

  test('session expired → logout clears auth and shows login', async ({ page }) => {
    await setup(page, { minipos: { session: 401 } });
    await page.goto('/');

    // API returns 401 for session, so the retry/logout button shows
    await expect(page.getByRole('button', { name: 'Изход' })).toBeVisible({ timeout: 8_000 });
    await page.getByRole('button', { name: 'Изход' }).click();

    // App shows auth screen
    await expect(page.getByText('Вход в BeeMiniPOS')).toBeVisible();
  });
});

test.describe('Offline mode', () => {
  test('backend unavailable during login → shows offline reference data if cached', async ({ page }) => {
    // First visit: populate cache
    await setup(page);
    await page.goto('/');
    await expect(page.getByText('Кафе')).toBeVisible({ timeout: 8_000 });

    // Second visit: all API calls fail, app uses IndexedDB reference cache
    await page.route('**/public/v1/minipos/**', (route) => route.abort('failed'));
    await page.reload();

    // Shows offline notice but still renders products from cache
    await expect(page.getByText(/Offline справочници/)).toBeVisible({ timeout: 8_000 });
    await expect(page.getByText('Кафе')).toBeVisible();
  });
});
