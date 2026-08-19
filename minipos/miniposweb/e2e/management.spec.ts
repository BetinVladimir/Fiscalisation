import { test, expect } from '@playwright/test';
import { setup, waitForPOS } from './helpers';

test.describe('Management – Products', () => {
  test('products list shows active products', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Товари' }).click();

    await expect(page.getByText('Товары')).toBeVisible();
    await expect(page.getByText('Кафе')).toBeVisible();
    await expect(page.getByText('Вода')).toBeVisible();
  });

  test('add new product', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Товари' }).click();
    await page.getByRole('button', { name: 'Добавить', exact: true }).click();

    await expect(page.getByText('Новый товар')).toBeVisible();

    await page.getByLabel('Название').fill('Чай');
    await page.getByLabel('Цена EUR').fill('1.50');
    await page.getByRole('button', { name: 'Сохранить' }).click();

    // After save, the form closes and the new product appears
    await expect(page.getByText('Нов продукт')).toBeVisible({ timeout: 5_000 });
  });

  test('cancel add product stays on list', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Товари' }).click();
    await page.getByRole('button', { name: 'Добавить', exact: true }).click();

    await expect(page.getByText('Новый товар')).toBeVisible();
    await page.getByRole('button', { name: 'Отмена' }).click();

    await expect(page.getByText('Кафе')).toBeVisible();
    await expect(page.getByText('Новый товар')).not.toBeVisible();
  });
});

test.describe('Management – Employees', () => {
  test('employees list shown', async ({ page }) => {
    await setup(page, {
      minipos: {
        employees: {
          items: [
            { id: 'emp-001', first_name: 'Иван', last_name: 'Иванов', operator_code: '001', roles: ['OPERATOR'] },
            { id: 'emp-002', first_name: 'Мария', last_name: 'Петрова', operator_code: '002', roles: ['OPERATOR'] },
          ],
        },
      },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Сотрудники' }).click();

    await expect(page.getByText('Сотрудники')).toBeVisible();
    await expect(page.getByText('Иван Иванов')).toBeVisible();
    await expect(page.getByText('Мария Петрова')).toBeVisible();
  });

  test('add new employee', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Сотрудники' }).click();
    await page.getByRole('button', { name: 'Добавить', exact: true }).click();

    await expect(page.getByText('Новый сотрудник')).toBeVisible();

    await page.getByLabel('Имя').fill('Петър');
    await page.getByLabel('Фамилия').fill('Петров');
    await page.getByLabel('Код оператора').fill('003');
    await page.getByRole('button', { name: 'Сохранить' }).click();

    await expect(page.getByText('Петър Петров')).toBeVisible({ timeout: 5_000 });
  });

  test('cancel add employee stays on list', async ({ page }) => {
    await setup(page, {
      minipos: { employees: { items: [{ id: 'emp-001', first_name: 'Иван', last_name: 'Иванов', operator_code: '001', roles: [] }] } },
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Сотрудники' }).click();
    await page.getByRole('button', { name: 'Добавить', exact: true }).click();
    await page.getByRole('button', { name: 'Отмена' }).click();

    await expect(page.getByText('Иван Иванов')).toBeVisible();
  });
});

test.describe('Navigation', () => {
  test('back to sale view from management', async ({ page }) => {
    await setup(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: 'Товари' }).click();
    await expect(page.getByText('Товары')).toBeVisible();

    await page.getByRole('button', { name: 'Продажби' }).click();
    await expect(page.getByText(/Текуща продажба/)).toBeVisible();
  });
});
