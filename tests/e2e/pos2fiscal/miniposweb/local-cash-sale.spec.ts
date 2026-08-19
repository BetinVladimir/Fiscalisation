/**
 * miniposweb – LOCAL_HTTP route, CASH sale via edge-agent-s3 (Datecs/Daisy boundary)
 *
 * When CLOUD is unavailable (fiscal-route-health 503) the RouteController falls over to
 * LOCAL_HTTP. The browser calls the edge-agent local HTTP API directly:
 *   GET  http://localhost:19001/beeloy/local/v1/healthz   — probe
 *   POST http://localhost:19001/beeloy/local/v1/intents   — submit FiscalIntent
 *
 * These tests run against the REAL device-mock.mjs server started in global-setup.ts on
 * port 19001 (edge-agent-s3 profile, CASH+FISCAL). After checkout, the test fetches
 * /__e2e/operations from the mock server to audit exactly what the browser sent.
 *
 * POS terminal control: minipos-backend routes are mocked via page.route.
 * Fiscal device control: the FiscalIntent is audited directly from the device mock.
 */

import { test, expect } from '@playwright/test';
import {
  setupMinipowsebRoutes,
  authenticate,
  waitForPOS,
  getLatestDeviceOperation,
  EDGE_AGENT_S3_BASE,
} from '../helpers';

test.describe('miniposweb – LOCAL_HTTP cash sale (edge-agent-s3)', () => {
  test('CLOUD down → LOCAL route submits FiscalIntent to edge-agent-s3 → FISCALIZED', async ({ page }) => {
    await setupMinipowsebRoutes(page, {
      // Disable CLOUD route → RouteController falls to LOCAL_HTTP
      fiscalRouteHealth: 503,
      // adapter_base_url points to real edge-agent-s3 mock (port 19001)
      adapterBase: EDGE_AGENT_S3_BASE,
    });
    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await expect(page.locator('.total')).toContainText('2.50');

    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();

    // Cart clears on FISCALIZED from device mock
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    // ── Fiscal device audit: verify what was sent to edge-agent-s3 ───────────
    const op = await getLatestDeviceOperation(19001);
    expect(op).toBeDefined();

    // Device processed the operation and returned FISCALIZED
    expect(op!.state).toBe('FISCALIZED');
    expect(op!.profile).toBe('edge-agent-s3');
    expect(op!.fiscal_reference).toMatch(/^E2E-edge-agent-s3-/);
  });

  test('device intent includes FiscalIntent structure: command, payload_sha256, items', async ({ page }) => {
    let capturedIntent: Record<string, unknown> | null = null;

    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      adapterBase: EDGE_AGENT_S3_BASE,
    });

    // Intercept the intent submission to capture the raw body BEFORE it reaches the device
    await page.route(`${EDGE_AGENT_S3_BASE}/intents`, async (route) => {
      capturedIntent = JSON.parse(route.request().postData() || '{}');
      // Forward to real device mock
      await route.continue();
    });

    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    // ── Verify FiscalIntent contract ─────────────────────────────────────────
    expect(capturedIntent).not.toBeNull();
    const intent = capturedIntent!;

    // Protocol fields
    expect(intent['protocol_version']).toBe('1.0');
    expect(intent['command']).toBe('SALE_FINALIZE');
    expect(intent['action']).toBe('SALE_FINALIZE');

    // payload_sha256: 64-char hex SHA-256 of canonical_payload
    expect(typeof intent['payload_sha256']).toBe('string');
    expect((intent['payload_sha256'] as string).length).toBe(64);
    expect(intent['payload_sha256']).toMatch(/^[0-9a-f]{64}$/);

    // client_operation_id (Idempotency-Key is set to this in local.submit())
    expect(typeof intent['client_operation_id']).toBe('string');
    expect((intent['client_operation_id'] as string).length).toBe(36); // UUID

    // intent_id present
    expect(typeof intent['intent_id']).toBe('string');

    // payments include CASH
    const payments = intent['payments'] as Array<Record<string, unknown>>;
    expect(Array.isArray(payments)).toBe(true);
    expect(payments.some((p) => p['type'] === 'CASH')).toBe(true);

    // items contain tax_group B (Кафе)
    const items = intent['items'] as Array<Record<string, unknown>>;
    expect(Array.isArray(items)).toBe(true);
    expect(items.length).toBeGreaterThan(0);
    expect(items[0]['tax_group']).toBe('B');
  });

  test('Idempotency-Key header matches client_operation_id in intent body', async ({ page }) => {
    let headerKey: string | null = null;
    let bodyKey: string | null = null;

    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      adapterBase: EDGE_AGENT_S3_BASE,
    });

    await page.route(`${EDGE_AGENT_S3_BASE}/intents`, async (route) => {
      headerKey = route.request().headers()['idempotency-key'] ?? null;
      const body = JSON.parse(route.request().postData() || '{}');
      bodyKey = body.client_operation_id ?? null;
      await route.continue();
    });

    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    expect(headerKey).not.toBeNull();
    expect(bodyKey).not.toBeNull();
    expect(headerKey).toBe(bodyKey);
  });

  test('device fiscal_reference shown in settings result panel', async ({ page }) => {
    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      adapterBase: EDGE_AGENT_S3_BASE,
    });
    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    // The result from the device (fiscal_reference starts with E2E-edge-agent-s3-) should appear
    await page.getByRole('button', { name: 'Настройки' }).click();
    await expect(page.locator('pre')).toContainText('E2E-edge-agent-s3-', { timeout: 5_000 });
    await expect(page.locator('pre')).toContainText('FISCALIZED');
  });

  test('binding_generation propagated from configuration to device intent', async ({ page }) => {
    let capturedBindingGen: unknown = null;

    await setupMinipowsebRoutes(page, {
      fiscalRouteHealth: 503,
      adapterBase: EDGE_AGENT_S3_BASE,
    });

    await page.route(`${EDGE_AGENT_S3_BASE}/intents`, async (route) => {
      const body = JSON.parse(route.request().postData() || '{}');
      capturedBindingGen = body.binding_generation;
      await route.continue();
    });

    await authenticate(page);
    await page.goto('/');
    await waitForPOS(page);

    await page.getByRole('button', { name: /Кафе/ }).click();
    await page.getByRole('button', { name: 'Плащане и фискализация' }).click();
    await expect(page.getByText('Изберете продукт.')).toBeVisible({ timeout: 20_000 });

    // binding_generation from configuration (= 1 in minipowsebConfig)
    expect(capturedBindingGen).toBe(1);
  });
});
