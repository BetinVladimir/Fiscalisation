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

  test('failed product save keeps entered values and shows Paper error', async ({ page }) => {
    await setup(page, { minipos: { createProduct: 500 } });
    await page.goto('/');
    await waitForPOS(page);
    await page.getByRole('button', { name: 'Товари' }).click();
    await page.getByTestId('management-add').click();
    await page.getByTestId('management-product-name').fill('Чай');
    await page.getByTestId('management-product-price').fill('1.50');
    await page.getByTestId('management-save').click();
    await expect(page.getByTestId('management-error')).toBeVisible();
    await expect(page.getByTestId('management-product-name')).toHaveValue('Чай');
    await expect(page.getByTestId('management-product-price')).toHaveValue('1.50');
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

    await expect(page.getByRole('heading', { name: 'Сотрудники' })).toBeVisible();
    await expect(page.locator('.entity-list').getByText('Иван Иванов')).toBeVisible();
    await expect(page.locator('.entity-list').getByText('Мария Петрова')).toBeVisible();
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

    await expect(page.locator('.entity-list').getByText('Иван Иванов')).toBeVisible();
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
