import { test, expect } from '@playwright/test';
import { setup, setupDeviceRoutes, setupMiniposRoutes, authenticate, waitForPOS } from './helpers';
import { translations } from '../src/ui/workTranslations.generated';

async function seedOutboxRecord(page: import('@playwright/test').Page, id: string, state: 'UNKNOWN' | 'FISCALIZED', pendingSync: boolean) {
  await page.evaluate(async ({ id, state, pendingSync }) => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open('beeloy-miniposweb', 1);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    const now = new Date().toISOString();
    const row = {
      intent: {
        protocol_version: '1.0', intent_id: id, action: 'SALE_FINALIZE', client_operation_id: id,
        receipt_session_id: 'receipt-e2e', client_sale_surrogate_id: 'external-e2e', server_sale_id: '',
        operator_code: '001', tenant_id: '00000000-0000-4000-8000-000000000001', location_id: 'loc-001', register_id: 'reg-001',
        edge_device_id: 'adapter-001', binding_generation: 1, command: 'SALE_FINALIZE',
        items: [{ item_id: 'line-001', name: 'Кафе', quantity: '1.000', unit_price: { amount: '2.50', currency: 'EUR' }, tax_group: 'B' }],
        payments: [{ payment_id: 'cash-001', type: 'CASH', amount: { amount: '2.50', currency: 'EUR' } }],
        metadata: {}, canonical_payload: {}, payload_sha256: 'e2e-fixture',
      },
      result: { operation_id: id, state, certainty: state === 'UNKNOWN' ? 'UNKNOWN' : 'PROVEN_SUCCESS', updated_at: now },
      posSync: pendingSync ? {
        shift_id: 'shift-001', external_id: 'external-e2e',
        lines: [{ line_id: 'line-001', product_id: 'prod-001', name: 'Кафе', quantity: '1.000', unit_price: { amount: '2.50', currency: 'EUR' }, tax_group: 'B' }],
        payments: [{ payment_id: 'cash-001', type: 'CASH', amount: { amount: '2.50', currency: 'EUR' } }],
      } : undefined,
      createdAt: now, updatedAt: now,
    };
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction('fiscal-outbox', 'readwrite');
      tx.objectStore('fiscal-outbox').put(row);
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
    db.close();
  }, { id, state, pendingSync });
}

async function readOutboxRecords(page: import('@playwright/test').Page) {
  return page.evaluate(async () => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open('beeloy-miniposweb', 1);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    const records = await new Promise<Array<{ result?: { state: string }; posSync?: { synced_at?: string } }>>((resolve, reject) => {
      const request = db.transaction('fiscal-outbox').objectStore('fiscal-outbox').getAll();
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    db.close();
    return records;
  });
}

test.describe('Offline outbox – cloud prepareOrder fails', () => {
  test('order creation fails → shows offline message, LOCAL_HTTP device still fiscalises', async ({ page }) => {
    let localSubmissions = 0;
    await setupMiniposRoutes(page, {
      createOrder: 503,  // cloud order creation unavailable
      fiscalLocalTokens: {
        access_token: 'local-adapter-token',
        expires_at: new Date(Date.now() + 30 * 60_000).toISOString(),
        adapter_base_url: 'http://localhost:19001/beeloy/local/v1',
      },
    });
    await setupDeviceRoutes(page);
    await authenticate(page);
    page.on('request', request => {
      if (request.url().includes('/beeloy/local/v1/intents') && request.method() === 'POST') localSubmissions++;
    });
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Offline cloud message shown
    await expect(
      page.getByText(/Cloud подготовката е недостъпна/),
    ).toBeVisible({ timeout: 10_000 });

    await expect.poll(() => localSubmissions).toBe(1);
    await expect.poll(async () => (await readOutboxRecords(page))[0]?.result?.state).toBe('FISCALIZED');
    expect(await readOutboxRecords(page)).toHaveLength(1);
  });

  test('outbox entry synced after syncOfflineOrders', async ({ page }) => {
    test.setTimeout(45_000);
    let allowImport = false;
    let importCalls = 0;
    let successfulImports = 0;
    let localSubmissions = 0;

    await setupMiniposRoutes(page, { createOrder: 503 });
    await setupDeviceRoutes(page);
    await authenticate(page);
    page.on('request', request => {
      if (request.url().includes('/beeloy/local/v1/intents') && request.method() === 'POST') localSubmissions++;
    });

    await page.route(/orders(?::|%3A)import-offline/i, async (route) => {
      importCalls++;
      if (!allowImport) return route.fulfill({ status: 503, body: JSON.stringify({ code: 'UNAVAILABLE' }), contentType: 'application/json' });
      successfulImports++;
      return route.fulfill({
        status: 200,
        body: JSON.stringify({ id: 'order-imported-001', state: 'ACCEPTED' }),
        contentType: 'application/json',
      });
    });

    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByTestId('sale-message')).toContainText('Cloud подготовката е недостъпна');
    await page.getByTestId('app-outbox').click();
    await page.getByTestId('offline-outbox-refresh').click();
    await expect(page.getByText(translations.bg.outboxPendingSync)).toBeVisible();
    await expect.poll(() => importCalls).toBeGreaterThanOrEqual(1);
    expect(localSubmissions).toBe(1);

    allowImport = true;
    await expect.poll(() => successfulImports, { timeout: 25_000 }).toBe(1);
    await page.getByTestId('offline-outbox-refresh').click();
    await expect(page.getByText(translations.bg.outboxSynced)).toBeVisible();
    expect(localSubmissions, 'sync must not resubmit the fiscal intent to the device').toBe(1);
    const rows = await readOutboxRecords(page);
    expect(rows).toHaveLength(1);
    const row = rows[0];
    expect(row.result?.state).toBe('FISCALIZED');
    expect(row.posSync?.synced_at).toBeTruthy();
    await page.reload();
    await waitForPOS(page);
    await page.getByTestId('app-outbox').click();
    await expect(page.getByText(translations.bg.outboxSynced)).toBeVisible();
    expect(localSubmissions, 'reload must not resubmit the fiscal intent').toBe(1);
  });
});

test.describe('Outbox – pre-existing UNKNOWN entry recovery', () => {
  test('previously UNKNOWN intent remains visible after reload without automatic device submission', async ({ page }) => {
    const clientOpId = '00000000-0000-4000-8000-000000000099';
    let deviceCalls = 0;
    await setupMiniposRoutes(page);
    await setupDeviceRoutes(page);
    await authenticate(page);
    page.on('request', request => {
      if (request.url().includes('/beeloy/local/v1/intents') && request.method() === 'POST') deviceCalls++;
    });
    await page.goto('/');
    await waitForPOS(page);
    await seedOutboxRecord(page, clientOpId, 'UNKNOWN', false);
    await page.reload();
    await waitForPOS(page);
    await page.getByTestId('app-outbox').click();
    await expect(page.getByTestId(`offline-outbox-status-${clientOpId}`)).toHaveText(translations.bg.outboxUncertain);
    expect(deviceCalls).toBe(0);
  });
});

test.describe('syncOfflineOrders – periodic sync', () => {
  test('pending posSync items are imported on page load', async ({ page }) => {
    const importedExternalIds: string[] = [];
    await setupMiniposRoutes(page);
    await setupDeviceRoutes(page);
    await authenticate(page);
    await page.route(/orders(?::|%3A)import-offline/i, async (route) => {
      const body = JSON.parse(route.request().postData() || '{}');
      importedExternalIds.push(body.external_id);
      return route.fulfill({
        status: 200,
        body: JSON.stringify({ id: `imported-${body.external_id}`, state: 'ACCEPTED' }),
        contentType: 'application/json',
      });
    });
    await page.goto('/');
    await waitForPOS(page);
    await seedOutboxRecord(page, '00000000-0000-4000-8000-000000000100', 'FISCALIZED', true);
    await page.reload();
    await waitForPOS(page);
    await expect.poll(() => importedExternalIds).toContain('external-e2e');
    await page.getByTestId('app-outbox').click();
    await expect(page.getByText(translations.bg.outboxSynced)).toBeVisible();
  });
});
