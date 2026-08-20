import { expect, test } from '@playwright/test';
import { DEVICE_ID, LOCATION_ID, OPERATOR_ID, REGISTER_ID, mockFiscalApi, openApp } from './helpers';

test('device diagnostics, printer and provisioning use public API with idempotency keys', async ({ page }) => {
  const mutations = await mockFiscalApi(page);
  await openApp(page);
  await page.getByTestId('device-diagnostics').click();
  await expect(page.getByTestId('diagnostics-result')).toContainText('secrets_redacted');
  await page.getByTestId('device-printer-test').click();
  await page.getByTestId('device-provision').click();
  await expect(page.getByTestId('provisioning-result')).toContainText('provision-1');
  const printer = mutations.find(x => x.path === `/devices/${DEVICE_ID}/tests/printer`)!;
  const provisioning = mutations.find(x => x.path === `/devices/${DEVICE_ID}/provisioning-sessions`)!;
  expect(printer.headers['idempotency-key']).toBeTruthy();
  expect(provisioning.headers['idempotency-key']).toBeTruthy();
  expect(printer.headers['x-api-version']).toBe('2026-08-07');
});

test('BLE session is bound to register, operator, app instance and prepared public key', async ({ page }) => {
  const mutations = await mockFiscalApi(page);
  await openApp(page);
  await page.getByLabel('Fiscal register ID').fill(REGISTER_ID);
  await page.getByTestId('ble-operator-id').fill(OPERATOR_ID);
  await page.getByTestId('ble-client-public-key').fill('x25519-public-key');
  await page.getByTestId('ble-session-issue').click();
  await expect(page.getByTestId('ble-session-result')).toContainText('ble-1');
  const issued = mutations.find(x => x.path === `/registers/${REGISTER_ID}/ble-sessions`)!;
  expect(issued.body.operator_id).toBe(OPERATOR_ID);
  expect(issued.body.public_key).toBe('x25519-public-key');
  expect(issued.body.app_instance_id).toMatch(/^beefiscal-admin-/);
});

test('SmartDevice activation requires lookup preview and sends tenant-scoped binding fields', async ({ page }) => {
  const mutations = await mockFiscalApi(page);
  await openApp(page);
  await page.getByLabel('Fiscal register ID').fill(REGISTER_ID);
  await page.getByTestId('smart-device-activation-code').fill('bc1234');
  await page.getByTestId('smart-device-activation-lookup').click();
  await expect(page.getByTestId('smart-device-activation-preview')).toContainText('BC-001');
  await page.getByTestId('bluecash-location-id').fill(LOCATION_ID);
  await page.getByTestId('bluecash-activate').click();
  await expect(page.getByTestId('bluecash-activation-result')).toContainText('ACTIVE');
  const confirmed = mutations.find(x => x.path === '/device-activation-requests/activation-1:confirm')!;
  expect(confirmed.body).toEqual({ user_code: 'BC1234', location_id: LOCATION_ID, register_id: REGISTER_ID, roles: ['PAYMENT_TERMINAL'] });
});

test('UNKNOWN operation is reconciled once and never re-submitted as a fiscal sale', async ({ page }) => {
  const mutations = await mockFiscalApi(page);
  await openApp(page);
  await page.getByLabel('Fiscal register ID').fill(REGISTER_ID);
  await page.getByTestId('tab-Операции').click();
  await expect(page.getByTestId('operation-unknown')).toBeVisible();
  await page.getByTestId('operation-reconcile-operation-unknown-1').click();
  await expect.poll(() => mutations.filter(x => x.path.endsWith('/reconcile')).length).toBe(1);
  expect(mutations.some(x => /\/sales/.test(x.path))).toBe(false);
});

test('report command and audit filters reach the API without client-side fabricated results', async ({ page }) => {
  const mutations = await mockFiscalApi(page);
  await openApp(page);
  await page.getByLabel('Fiscal register ID').fill(REGISTER_ID);
  await page.getByTestId('tab-Отчети').click();
  await page.getByLabel('Създай Z отчет').click();
  await expect.poll(() => mutations.some(x => x.path === `/registers/${REGISTER_ID}/reports` && x.body.type === 'Z')).toBe(true);
  await page.getByTestId('tab-Одит').click();
  await page.getByLabel('Действие').fill('SALE_FINALIZED');
  await page.getByLabel('УНП').fill('UNP-1');
  const filteredAudit = page.waitForRequest(r => r.url().includes('/audit-events?action=SALE_FINALIZED') && r.url().includes('unp=UNP-1'));
  await page.getByTestId('apply-audit-filters').click();
  const auditRequest = await filteredAudit;
  await expect(page.getByTestId('audit-event-audit-1')).toContainText('SALE_FINALIZED');
  expect(auditRequest.method()).toBe('GET');
});

test('backend failure is fail-closed and exposes CORE OFFLINE instead of stale device controls', async ({ page }) => {
  await mockFiscalApi(page, { devicesStatus: 503 });
  await page.goto('/');
  await expect(page.getByTestId('status-fiscal-device')).toContainText('CORE OFFLINE');
  await expect(page.getByText(/API отказ/)).toBeVisible();
  await expect(page.getByTestId('device-diagnostics')).toBeDisabled();
});
