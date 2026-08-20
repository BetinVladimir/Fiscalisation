import type { Page, Route } from '@playwright/test';

// ── shared auth constants ────────────────────────────────────────────────────

export const TEST_JWT =
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9' +
  '.eyJ0ZW5hbnRfaWQiOiIwMDAwMDAwMC0wMDAwLTQwMDAtODAwMC0wMDAwMDAwMDAwMDEiLCJzdWIiOiJ1c2VyLTEiLCJleHAiOjk5OTk5OTk5OTl9' +
  '.e2e_test';

export const TEST_REFRESH = 'refresh-pos2fiscal-e2e';

// ── Datecs/Daisy device boundary URLs ───────────────────────────────────────
// edge-agent-s3 handles CASH+FISCAL on port 19001
// bluecash-app handles CARD+FISCAL on port 19002

export const EDGE_AGENT_S3_ORIGIN = 'http://localhost:19001';
export const BLUECASH_APP_ORIGIN  = 'http://localhost:19002';
export const EDGE_AGENT_S3_BASE   = `${EDGE_AGENT_S3_ORIGIN}/beeloy/local/v1`;
export const BLUECASH_APP_BASE    = `${BLUECASH_APP_ORIGIN}/beeloy/local/v1`;

// ── device audit ─────────────────────────────────────────────────────────────

export type DeviceOperation = {
  operation_id: string;
  intent_id: string;
  client_operation_id: string;
  state: string;
  profile: string;
  payment_type: string;
  fiscal_reference: string;
  binding_generation: number;
  completed_at: string;
  // card-specific (bluecash-app only)
  payment_reference?: string;
  rrn?: string;
  authorization_code?: string;
};

export async function getDeviceOperations(port: 19001 | 19002): Promise<DeviceOperation[]> {
  const res = await fetch(`http://localhost:${port}/__e2e/operations`);
  const data: { items: DeviceOperation[] } = await res.json();
  return data.items;
}

export async function getLatestDeviceOperation(
  port: 19001 | 19002,
): Promise<DeviceOperation | undefined> {
  const items = await getDeviceOperations(port);
  return items.at(-1);
}

// ── response shorthand ───────────────────────────────────────────────────────

function respond(route: Route, value: object | number, status = 200) {
  if (typeof value === 'number') {
    return route.fulfill({
      status: value,
      body: JSON.stringify({ code: 'ERROR' }),
      contentType: 'application/json',
    });
  }
  return route.fulfill({
    status,
    body: JSON.stringify(value),
    contentType: 'application/json',
  });
}

// ── miniposweb mock data ─────────────────────────────────────────────────────

export const MINIPOSWEB_EMPLOYEE = {
  id: 'emp-001',
  first_name: 'Иван',
  last_name: 'Иванов',
  operator_code: '001',
  roles: ['OPERATOR'],
};

export const MINIPOSWEB_SESSION = {
  authenticated: true,
  employee: MINIPOSWEB_EMPLOYEE,
  expires_at: '2099-12-31T23:59:59Z',
};

export const MINIPOSWEB_SHIFT = {
  id: 'shift-001',
  register_id: 'reg-001',
  employee_id: 'emp-001',
  state: 'OPEN',
  version: 1,
};

export const MINIPOSWEB_ORDER = {
  id: 'order-p2f-001',
  external_id: 'ext-p2f-001',
  version: 1,
};

export const MINIPOSWEB_LOCAL_TOKEN = {
  access_token: 'local-adapter-token-p2f',
  expires_at: '2099-12-31T23:59:59Z',
  adapter_base_url: EDGE_AGENT_S3_BASE,
};

export function minipowsebConfig(adapterBase = EDGE_AGENT_S3_BASE) {
  return {
    location_id: 'loc-001',
    fiscal_register_id: 'reg-001',
    fiscal_adapter_id: 'adapter-001',
    binding_generation: 1,
    adapter_base_url: adapterBase,
  };
}

// ── miniposweb route mock ────────────────────────────────────────────────────

export type MinipowsebOverrides = {
  fiscalRouteHealth?: object | number;
  checkoutBatch?: object | number;
  adapterBase?: string;
};

export async function setupMinipowsebRoutes(
  page: Page,
  overrides: MinipowsebOverrides = {},
) {
  await page.route('**/public/v1/minipos/**', async (route) => {
    const url = route.request().url();
    const method = route.request().method();

    if (url.includes('/auth/refresh'))
      return respond(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600 });

    if (url.includes('/auth/'))
      return respond(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600, onboarding_required: false });

    if (url.includes('/operator-session'))
      return respond(route, MINIPOSWEB_SESSION);

    if (url.includes('/configuration'))
      return respond(route, minipowsebConfig(overrides.adapterBase));

    if (url.includes('/fiscal-route-health'))
      return respond(route, overrides.fiscalRouteHealth ?? { status: 'ok' });

    if (url.includes('/fiscal-local-tokens'))
      return respond(route, {
        ...MINIPOSWEB_LOCAL_TOKEN,
        adapter_base_url: overrides.adapterBase ?? EDGE_AGENT_S3_BASE,
      });

    if (url.includes('/shifts') && method === 'GET')
      return respond(route, { items: [MINIPOSWEB_SHIFT] });

    if (url.includes('/shifts') && method === 'POST')
      return respond(route, MINIPOSWEB_SHIFT);

    if (url.includes('/products'))
      return respond(route, {
        items: [
          { id: 'prod-001', name: 'Кафе', price: { amount: '2.50', currency: 'EUR' }, tax_group: 'B', status: 'ACTIVE' },
          { id: 'prod-002', name: 'Вода', price: { amount: '1.00', currency: 'EUR' }, tax_group: 'B', status: 'ACTIVE' },
        ],
      });

    if (url.includes('/employees') && method === 'GET')
      return respond(route, { items: [] });

    if (url.includes('/checkout-batch') && method === 'POST')
      return respond(
        route,
        overrides.checkoutBatch ?? {
          operation_id: 'op-cloud-p2f-001',
          state: 'FISCALIZED',
          fiscal_reference: 'CLOUD-P2F-REF-001',
          updated_at: new Date().toISOString(),
        },
      );

    if (url.includes('/orders:import-offline') || url.includes('/orders%3Aimport-offline'))
      return respond(route, { id: 'order-imported', state: 'ACCEPTED' });

    if (url.includes('/reversals') && method === 'POST')
      return respond(route, { operation_id: 'op-rev-001', state: 'FISCALIZED', fiscal_reference: 'REV-001', updated_at: new Date().toISOString() });

    if (url.includes('/lines') && method === 'POST')
      return respond(route, { id: 'line-001', version: 2 });

    if (url.includes('/orders') && method === 'POST')
      return respond(route, MINIPOSWEB_ORDER);

    await route.fulfill({ status: 404, body: JSON.stringify({ code: 'NOT_FOUND' }), contentType: 'application/json' });
  });
}

// ── BeeMiniPOS mock data ─────────────────────────────────────────────────────

export const REGISTER_ID = '00000000-0000-4000-8000-000000000001';

export const BEEMINIPOS_PRODUCTS = [
  { id: 'coffee', sku: 'COF', barcode: '380000000001', name: 'Кафе', price: { amount: '2.50', currency: 'EUR' }, tax_group: 'B', active: true },
  { id: 'water',  sku: 'WAT', barcode: '380000000002', name: 'Вода', price: { amount: '1.00', currency: 'EUR' }, tax_group: 'B', active: true },
];

export const BEEMINIPOS_CONFIGURATION = {
  id: 'config-1',
  location_name: 'Магазин 1',
  location_address: 'София',
  workstation_name: 'Каса 01',
  fiscal_register_id: REGISTER_ID,
  version: 1,
};

export const BEEMINIPOS_OPEN_SHIFT = {
  id: 'shift-1',
  register_id: REGISTER_ID,
  employee_id: 'employee-1',
  state: 'OPEN',
  allowed_actions: ['SALE', 'CLOSE'],
};

export const BEEMINIPOS_SESSION = {
  session_id: '00000000-0000-4000-8000-000000000020',
  workstation_id: REGISTER_ID,
  operator_code: '0001',
  expires_at: '2099-12-31T23:59:59Z',
};

let _saleCounter = 0;
let _lastSale: Record<string, unknown> = {};

export function resetFiscalSaleState() {
  _saleCounter = 0;
  _lastSale = {};
}

export function makeFiscalSale(overrides: Record<string, unknown> = {}) {
  _saleCounter++;
  const unp = `${REGISTER_ID}-0001-${String(_saleCounter).padStart(7, '0')}`;
  return {
    sale_id: `sale-p2f-${_saleCounter}`,
    external_id: `ext-p2f-${_saleCounter}`,
    register_id: REGISTER_ID,
    operator_id: '0001',
    unp,
    regulatory_identifiers: [{
      type: 'SALE',
      scheme: 'BG_UNP_V1',
      value: unp,
      country_code: 'BG',
      profile_version: '2026-08-10.1',
    }],
    state: 'OPEN',
    version: 1,
    lines: [],
    payments: [],
    allowed_actions: ['ADD_LINE', 'PAY', 'CANCEL'],
    totals: { gross: { amount: '0.00', currency: 'EUR' } },
    ...overrides,
  };
}

// ── BeeMiniPOS route mocks ───────────────────────────────────────────────────

const MINIPOS_API = 'http://localhost:8081/public/v1/minipos';
const FISCAL_API  = 'http://localhost:8080/public/v1';
const FISCAL_PING = 'http://localhost:8080/connectivity/ping';

export type BeeMinipossOverrides = {
  finalizeResponse?: object | number;
  finalizeState?: 'FISCALIZED' | 'UNKNOWN' | 'FAILED';
  onFinalize?: (body: Record<string, unknown>, headers: Record<string, string>) => void;
  onOpenWithLine?: (body: Record<string, unknown>) => void;
};

export async function setupBeeMiniposRoutes(
  page: Page,
  overrides: BeeMinipossOverrides = {},
) {
  resetFiscalSaleState();

  // HEAD /connectivity/ping — pingFiscal expects 204 + X-BeeFiscal-Ping: 1
  await page.route(FISCAL_PING, (route) => {
    if (route.request().method() === 'HEAD')
      return route.fulfill({ status: 204, headers: { 'X-BeeFiscal-Ping': '1' } });
    return route.continue();
  });

  // minipos-backend
  await page.route(`${MINIPOS_API}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    const method = route.request().method();

    if (path.endsWith('/auth/refresh'))
      return respond(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600 });

    if (path.endsWith('/products') && method === 'GET')
      return respond(route, { items: BEEMINIPOS_PRODUCTS, page: { has_more: false } });

    if (path.endsWith('/tax-groups') && method === 'GET')
      return respond(route, { items: [{ id: 'tg-b', code: 'B', name: 'Стандартна', rate: '20', status: 'ACTIVE', version: 1 }], page: { has_more: false } });

    if (path.endsWith('/employees') && method === 'GET')
      return respond(route, { items: [{ id: 'employee-1', first_name: 'Иван', last_name: 'Петров', operator_code: '0001', roles: ['OPERATOR'] }], page: { has_more: false } });

    if (path.endsWith('/configuration') && method === 'GET')
      return respond(route, BEEMINIPOS_CONFIGURATION);

    if (path.endsWith('/shifts') && method === 'GET')
      return respond(route, { items: [BEEMINIPOS_OPEN_SHIFT], page: { has_more: false } });

    await route.fulfill({ status: 404, body: JSON.stringify({ code: 'NOT_FOUND', path }), contentType: 'application/json' });
  });

  // fiscal-backend (Datecs/Daisy boundary from BeeMiniPOS perspective)
  await page.route(`${FISCAL_API}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    const method = route.request().method();

    if (path.endsWith('/clock-sync') && method === 'POST')
      return respond(route, { state: 'VERIFIED' });

    if (path.endsWith('/readiness:refresh') && method === 'POST')
      return respond(route, { ready: true });

    if (path.endsWith('/sessions') && method === 'POST')
      return respond(route, BEEMINIPOS_SESSION, 201);

    if (path.endsWith('/sales') && method === 'GET') {
      const items = _lastSale?.state === 'OPEN' ? [_lastSale] : [];
      return respond(route, { items, page: { has_more: false } });
    }

    if (path.endsWith('/sales:open-with-line') && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      overrides.onOpenWithLine?.(body);
      _lastSale = makeFiscalSale({
        external_id: body.client_sale_surrogate_id,
        lines: [body.line],
        totals: { gross: body.line?.unit_price ?? { amount: '0.00', currency: 'EUR' } },
      });
      return respond(route, _lastSale, 201);
    }

    if (/\/sales\/[^/]+\/lines$/.test(path) && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      (_lastSale.lines as unknown[]).push(body);
      _lastSale = { ..._lastSale, version: Number(_lastSale.version) + 1 };
      return respond(route, _lastSale, 201);
    }

    if (/\/sales\/[^/]+:finalize$/.test(path) && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const headers = route.request().headers();
      overrides.onFinalize?.(body, headers);

      if (typeof overrides.finalizeResponse === 'number')
        return respond(route, { code: 'ERROR' }, overrides.finalizeResponse);

      const state = overrides.finalizeState ?? 'FISCALIZED';
      if (state === 'FISCALIZED')
        _lastSale = { ..._lastSale, state: 'COMPLETED', version: Number(_lastSale.version) + 1 };

      return respond(
        route,
        overrides.finalizeResponse ?? {
          operation_id: `fiscal-p2f-${_saleCounter}`,
          type: 'FISCAL_SALE',
          state,
          fiscal_reference: state === 'FISCALIZED' ? `FD-P2F-${_saleCounter}` : null,
          allowed_actions: [],
        },
        202,
      );
    }

    if (/\/sales\/[^/]+$/.test(path) && method === 'GET')
      return respond(route, _lastSale);

    if (/\/operations\/[^/:]+$/.test(path) && method === 'GET')
      return respond(route, {
        operation_id: `fiscal-p2f-${_saleCounter}`,
        type: 'FISCAL_SALE',
        state: 'FISCALIZED',
        fiscal_reference: `FD-P2F-${_saleCounter}`,
        allowed_actions: [],
      });

    if (/\/registers\/[^/]+$/.test(path))
      return respond(route, { id: REGISTER_ID, type: 'FISCAL_REGISTER', device_id: 'device-1' });

    if (path.endsWith('/workstations') || path.includes('/workstations/'))
      return respond(route, {});

    await route.fulfill({ status: 404, body: JSON.stringify({ code: 'NOT_FOUND', path }), contentType: 'application/json' });
  });
}

// ── auth bypass (both apps use same localStorage keys) ───────────────────────

export async function authenticate(page: Page) {
  await page.addInitScript(
    ([tok, ref]) => {
      localStorage.setItem('minipos-access-token', tok);
      localStorage.setItem('minipos-refresh-token', ref);
    },
    [TEST_JWT, TEST_REFRESH] as const,
  );
}

// ── app-specific readiness wait ──────────────────────────────────────────────

export async function waitForPOS(page: Page) {
  await page.getByText('Кафе').waitFor({ timeout: 12_000 });
}

export async function waitForBeeMiniposReady(page: Page) {
  await page.getByTestId('status-transport')
    .filter({ hasText: /Готово|Смяната е отворена|Отворената смяна е възстановена/ })
    .waitFor({ timeout: 20_000 });
}
