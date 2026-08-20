import type { Page, Route } from '@playwright/test';

export const REGISTER_ID = '00000000-0000-4000-8000-000000000001';
export const LOCATION_ID = '00000000-0000-4000-8000-000000000002';
export const DEVICE_ID = '00000000-0000-4000-8000-000000000003';
export const OPERATOR_ID = '00000000-0000-4000-8000-000000000004';
const API = 'http://fiscal-admin.test/public/v1';

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
}

export type CapturedMutation = { path: string; body: Record<string, unknown>; headers: Record<string, string> };

export async function mockFiscalApi(page: Page, options: { devicesStatus?: number } = {}) {
  const mutations: CapturedMutation[] = [];
  await page.route(`${API}/**`, async route => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace('/public/v1', '') + url.search;
    const method = request.method();
    const body = request.postData() ? JSON.parse(request.postData()!) : {};
    if (method !== 'GET') mutations.push({ path, body, headers: request.headers() });

    if (path.startsWith('/devices?') && method === 'GET') {
      if (options.devicesStatus) return json(route, { code: 'UNAVAILABLE' }, options.devicesStatus);
      return json(route, { items: [{ id: DEVICE_ID, name: 'Datecs counter', vendor: 'Datecs', model: 'DP-150 MX', status: 'ACTIVE', transport: 'RS232' }], page: { has_more: false } });
    }
    if (path === `/devices/${DEVICE_ID}/health`) return json(route, { effective_state: 'READY', observed_at: '2026-08-20T10:00:00Z' });
    if (path === `/devices/${DEVICE_ID}/activity`) return json(route, { items: [{ state: 'READY', at: '2026-08-20T10:00:00Z' }] });
    if (path === `/devices/${DEVICE_ID}/capabilities`) return json(route, { printer_test: true, ble: true });
    if (path === `/devices/${DEVICE_ID}/diagnostics`) return json(route, { driver: 'datecs', secrets_redacted: true });
    if (path === `/devices/${DEVICE_ID}/tests/printer` && method === 'POST') return json(route, { id: 'printer-1', state: 'COMPLETED' }, 202);
    if (path === `/devices/${DEVICE_ID}/provisioning-sessions` && method === 'POST') return json(route, { session_id: 'provision-1', state: 'PENDING' }, 201);
    if (path === `/devices/${DEVICE_ID}:disconnect` && method === 'POST') return json(route, { state: 'DISCONNECTED' });
    if (path === `/registers/${REGISTER_ID}/ble-sessions` && method === 'POST') return json(route, { ble_session_id: 'ble-1', expires_at: '2026-08-20T11:00:00Z' }, 201);
    if (path.startsWith('/device-activation-requests:lookup')) return json(route, { activation_request_id: 'activation-1', vendor: 'BlueCash', model: 'A920', serial: 'BC-001', requested_roles: ['PAYMENT_TERMINAL'] });
    if (path === '/device-activation-requests/activation-1:confirm' && method === 'POST') return json(route, { device_id: DEVICE_ID, state: 'ACTIVE' });
    if (path.startsWith('/operations') && method === 'GET') return json(route, { items: [{ id: 'operation-unknown-1', type: 'FISCAL_SALE', state: 'UNKNOWN', unp: 'UNP-1', allowed_actions: ['RECONCILE'] }], page: { has_more: false } });
    if (path === '/operations/operation-unknown-1/reconcile' && method === 'POST') return json(route, { operation_id: 'operation-unknown-1', state: 'FISCALIZED' });
    if (path.startsWith('/reports') || path.includes(`/registers/${REGISTER_ID}/reports`)) {
      if (method === 'POST') return json(route, { id: 'report-z-1', state: 'COMPLETED', fiscal_reference: 'Z-REF-1' }, 202);
      return json(route, { items: [{ id: 'report-1', type: 'X', state: 'COMPLETED', fiscal_reference: 'X-REF-1' }], page: { has_more: false } });
    }
    if (path.startsWith('/audit-events')) return json(route, { items: [{ event_id: 'audit-1', action: 'SALE_FINALIZED', object_type: 'SALE', object_id: 'sale-1', actor_id: OPERATOR_ID, occurred_at: '2026-08-20T10:00:00Z', unp: 'UNP-1', event_hash: 'abc123' }], page: { has_more: false } });
    if (path.startsWith('/locations?')) return json(route, { items: [{ id: LOCATION_ID, code: 'SOF', name: 'Sofia' }], page: { has_more: false } });
    if (path.startsWith('/registers?')) return json(route, { items: [{ id: REGISTER_ID, code: 'R01', location_id: LOCATION_ID, version: 1 }], page: { has_more: false } });
    if (path.startsWith('/operators?')) return json(route, { items: [{ id: OPERATOR_ID, code: 'A001', first_name: 'Ivan', last_name: 'Ivanov' }], page: { has_more: false } });
    if (path === `/registers/${REGISTER_ID}/composite-bindings`) return json(route, { items: [], page: { has_more: false } });
    if (path === '/country-policy') return json(route, { country: 'BG', currency: 'EUR' });
    if (path === '/tax-groups') return json(route, { items: [{ code: 'B', rate: '20' }] });
    return json(route, { code: 'NOT_FOUND', path }, 404);
  });
  return mutations;
}

export async function openApp(page: Page) {
  await page.goto('/');
  await page.getByTestId('status-fiscal-device').filter({ hasText: 'CORE READY' }).waitFor({ timeout: 10_000 });
}
