import { test, expect } from '@playwright/test';
import { setup, waitForReady, OPEN_SHIFT, REGISTER_ID } from './helpers';

test.describe('Shift – open', () => {
  test('no shift → open shift → OPEN state, catalog available', async ({ page }) => {
    let shiftOpened = false;

    await setup(page, { minipos: { shifts: { items: [], page: { has_more: false } } } });

    await page.route('**/public/v1/minipos/shifts', async (route) => {
      if (route.request().method() === 'POST') {
        shiftOpened = true;
        return route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ ...OPEN_SHIFT, id: 'shift-new-1' }),
        });
      }
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: shiftOpened ? [OPEN_SHIFT] : [], page: { has_more: false } }),
      });
    });

    await page.goto('/');
    await page.getByTestId('status-transport').filter({ hasText: /Готово • отворете смяна/ }).waitFor({ timeout: 15_000 });

    await page.getByTestId('shift-toggle').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Смяната е отворена/ })
      .waitFor({ timeout: 10_000 });

    expect(shiftOpened).toBe(true);
    await expect(page.getByTestId('product-coffee')).toBeVisible();
  });

  test('no configuration → cannot open shift, status shows message', async ({ page }) => {
    await setup(page, {
      minipos: {
        configuration: 404,
        shifts: { items: [], page: { has_more: false } },
      },
    });
    await page.goto('/');

    await page.getByTestId('status-transport')
      .filter({ hasText: /Настройте валидна фискална каса/ })
      .waitFor({ timeout: 15_000 });

    // Attempting shift toggle on invalid register
    await page.getByTestId('shift-toggle').click();
    await page.getByTestId('status-transport')
      .filter({ hasText: /неуспешно|Настройте/ })
      .waitFor({ timeout: 5_000 });
  });
});

test.describe('Shift – close with Z-report', () => {
  test('close shift → CLOSED with z_fiscal_reference', async ({ page }) => {
    let closeCalled = false;
    await setup(page);

    await page.route(/\/shifts\/[^/]+\/close$/, async (route) => {
      closeCalled = true;
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'shift-1',
          register_id: REGISTER_ID,
          employee_id: 'employee-1',
          state: 'CLOSED',
          allowed_actions: [],
          z_operation_id: 'z-op-1',
          z_fiscal_reference: 'Z-000001',
        }),
      });
    });

    await page.goto('/');
    await waitForReady(page);

    await page.getByTestId('shift-toggle').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Z-000001/ })
      .waitFor({ timeout: 10_000 });

    expect(closeCalled).toBe(true);
  });

  test('BLOCKED_RECONCILIATION → shift-toggle shows "Свери Z"', async ({ page }) => {
    await setup(page, {
      minipos: {
        shifts: {
          items: [{
            id: 'shift-1',
            register_id: REGISTER_ID,
            employee_id: 'employee-1',
            state: 'BLOCKED_RECONCILIATION',
            allowed_actions: ['READ', 'RECONCILE'],
            z_operation_id: 'z-op-1',
            close_error: 'UNKNOWN',
          }],
          page: { has_more: false },
        },
      },
    });

    await page.goto('/');

    await page.getByTestId('status-transport')
      .filter({ hasText: /Z-отчетът изисква сверяване/ })
      .waitFor({ timeout: 15_000 });

    await expect(page.getByTestId('shift-toggle')).toContainText(/Свери Z/);
  });

  test('BLOCKED_RECONCILIATION reconcile → CLOSED, only one reconcile call', async ({ page }) => {
    let reconcileCalls = 0;

    await setup(page, {
      minipos: {
        shifts: {
          items: [{
            id: 'shift-1',
            register_id: REGISTER_ID,
            employee_id: 'employee-1',
            state: 'BLOCKED_RECONCILIATION',
            allowed_actions: ['READ', 'RECONCILE'],
          }],
          page: { has_more: false },
        },
      },
    });

    await page.route(/\/shifts\/[^/]+\/reconcile$/, async (route) => {
      reconcileCalls++;
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'shift-1',
          state: 'CLOSED',
          allowed_actions: [],
          z_operation_id: 'z-op-1',
          z_fiscal_reference: 'Z-000001',
        }),
      });
    });

    await page.goto('/');
    await page.getByTestId('status-transport')
      .filter({ hasText: /Z-отчетът изисква сверяване/ })
      .waitFor({ timeout: 15_000 });

    await page.getByTestId('shift-toggle').click();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Z-000001/ })
      .waitFor({ timeout: 10_000 });

    expect(reconcileCalls).toBe(1);
  });
});

test.describe('Shift – startup recovery', () => {
  test('page reload with open shift → status shows recovery message', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForReady(page);

    await page.reload();

    await page.getByTestId('status-transport')
      .filter({ hasText: /Отворената смяна е възстановена/ })
      .waitFor({ timeout: 15_000 });
  });

  test('missing configuration on reload → does not recover shift', async ({ page }) => {
    let shiftCallCount = 0;
    await setup(page, { minipos: { configuration: 404 } });

    await page.route('**/public/v1/minipos/shifts**', async (route) => {
      shiftCallCount++;
      await route.continue();
    });

    await page.goto('/');

    await page.getByTestId('status-transport')
      .filter({ hasText: /Настройте валидна фискална каса/ })
      .waitFor({ timeout: 15_000 });

    expect(shiftCallCount).toBe(0);
  });
});
