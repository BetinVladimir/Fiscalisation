import type { Page, Route } from '@playwright/test';

// ── API base URLs (baked into the Expo web bundle at default values) ────────
export const MINIPOS_API = 'http://localhost:8081/public/v1/minipos';
export const FISCAL_API  = 'http://localhost:8080/public/v1';
export const FISCAL_PING = 'http://localhost:8080/connectivity/ping';

// ── Test JWT (tenant_id extracted by tenantFrom()) ──────────────────────────
export const TEST_JWT =
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9' +
  '.eyJ0ZW5hbnRfaWQiOiIwMDAwMDAwMC0wMDAwLTQwMDAtODAwMC0wMDAwMDAwMDAwMDEiLCJzdWIiOiJ1c2VyLTEiLCJleHAiOjk5OTk5OTk5OTl9' +
  '.e2e_test';
export const TEST_REFRESH = 'refresh-e2e-bmp';

// ── Canonical mock data ──────────────────────────────────────────────────────

export const REGISTER_ID = '00000000-0000-4000-8000-000000000001';

export const PRODUCTS = [
  { id: 'coffee', sku: 'COF', barcode: '380000000001', name: 'Кафе', price: { amount: '2.50', currency: 'EUR' }, tax_group: 'B', active: true },
  { id: 'water',  sku: 'WAT', barcode: '380000000002', name: 'Вода', price: { amount: '1.00', currency: 'EUR' }, tax_group: 'B', active: true },
];

export const TAX_GROUPS = [
  { id: 'tg-b', code: 'B', name: 'Стандартна', rate: '20', status: 'ACTIVE', version: 1 },
];

export const EMPLOYEES = [
  { id: 'employee-1', first_name: 'Иван', last_name: 'Петров', operator_code: '0001', roles: ['OPERATOR'] },
];

export const CONFIGURATION = {
  id: 'config-1',
  location_name: 'Магазин 1',
  location_address: 'София',
  workstation_name: 'Каса 01',
  fiscal_register_id: REGISTER_ID,
  version: 1,
};

export const WORKSTATION_SESSION = {
  session_id: '00000000-0000-4000-8000-000000000020',
  workstation_id: REGISTER_ID,
  operator_code: '0001',
  expires_at: '2099-12-31T23:59:59Z',
};

export const OPEN_SHIFT = {
  id: 'shift-1',
  register_id: REGISTER_ID,
  employee_id: 'employee-1',
  state: 'OPEN',
  allowed_actions: ['SALE', 'CLOSE'],
};

export function makeFiscalSale(overrides: Record<string, unknown> = {}) {
  const num = Math.floor(Math.random() * 9000) + 1000;
  return {
    sale_id: `sale-${num}`,
    external_id: `ext-${num}`,
    register_id: REGISTER_ID,
    operator_id: '0001',
    unp: `AB123456-0001-${String(num).padStart(7, '0')}`,
    regulatory_identifiers: [{
      type: 'SALE',
      scheme: 'BG_UNP_V1',
      value: `AB123456-0001-${String(num).padStart(7, '0')}`,
      country_code: 'BG',
      profile_version: '2026.1',
    }],
    state: 'OPEN',
    version: 1,
    lines: [],
    payments: [],
    allowed_actions: ['ADD_LINE', 'CANCEL_LINE', 'PAY', 'CANCEL'],
    totals: { gross: { amount: '0.00', currency: 'EUR' } },
    ...overrides,
  };
}

// ── Route mock helpers ───────────────────────────────────────────────────────

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, body: JSON.stringify(body), contentType: 'application/json' });
}

export type MiniposOverrides = {
  products?: unknown;
  taxGroups?: unknown;
  employees?: unknown;
  configuration?: unknown | number;
  shifts?: unknown;
  openShift?: unknown;
  orders?: unknown;
  reverseOrder?: unknown;
  salesReport?: unknown;
};

/**
 * Mocks the MiniPOS backend (http://localhost:8081/public/v1/minipos).
 */
export async function setupMiniposRoutes(page: Page, overrides: MiniposOverrides = {}) {
  await page.route(`${MINIPOS_API}/**`, async (route) => {
    const url = route.request().url();
    const method = route.request().method();
    const path = new URL(url).pathname;

    if (path.endsWith('/products') && method === 'GET')
      return json(route, overrides.products ?? { items: PRODUCTS, page: { has_more: false } });

    if (path.endsWith('/tax-groups') && method === 'GET')
      return json(route, overrides.taxGroups ?? { items: TAX_GROUPS, page: { has_more: false } });

    if (path.endsWith('/employees') && method === 'GET')
      return json(route, overrides.employees ?? { items: EMPLOYEES, page: { has_more: false } });

    if (path.endsWith('/employees') && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      return json(route, { id: 'employee-new', ...body }, 201);
    }

    if (path.endsWith('/configuration') && method === 'GET') {
      const val = overrides.configuration;
      return typeof val === 'number' ? json(route, { code: 'NOT_FOUND' }, val) : json(route, val ?? CONFIGURATION);
    }

    if (path.endsWith('/configuration') && method === 'PATCH') {
      const body = JSON.parse(route.request().postData() || '{}');
      return json(route, { ...CONFIGURATION, ...body, version: CONFIGURATION.version + 1 });
    }

    if (path.endsWith('/shifts') && method === 'GET')
      return json(route, overrides.shifts ?? { items: [OPEN_SHIFT], page: { has_more: false } });

    if (path.endsWith('/shifts') && method === 'POST')
      return json(route, overrides.openShift ?? OPEN_SHIFT, 201);

    if (/\/shifts\/[^/]+\/close$/.test(path) && method === 'POST')
      return json(route, { ...OPEN_SHIFT, state: 'CLOSED', allowed_actions: [], z_operation_id: 'z-op-1', z_fiscal_reference: 'Z-000001' });

    if (/\/shifts\/[^/]+\/reconcile$/.test(path) && method === 'POST')
      return json(route, { ...OPEN_SHIFT, state: 'CLOSED', allowed_actions: [], z_operation_id: 'z-op-1', z_fiscal_reference: 'Z-000001' });

    if (path.endsWith('/orders') && method === 'GET')
      return json(route, overrides.orders ?? { items: [], page: { has_more: false } });

    if (/\/orders\/[^/]+\/reversals$/.test(path) && method === 'POST')
      return json(route, overrides.reverseOrder ?? { operation_id: 'rev-op-1', state: 'FISCALIZED', fiscal_reference: 'REV-001' }, 202);

    if (path.endsWith('/reports/sales') && method === 'GET')
      return json(route, overrides.salesReport ?? {
        from: '2026-01-01T00:00:00Z',
        to: '2026-08-19T23:59:59Z',
        currency: 'EUR',
        gross: { amount: '250.00', currency: 'EUR' },
        payments: [{ type: 'CASH', amount: { amount: '250.00', currency: 'EUR' } }],
        generated_at: '2026-08-19T12:00:00Z',
      });

    if (path.endsWith('/auth/request-code') && method === 'POST')
      return route.fulfill({ status: 204, body: '' });

    if (path.endsWith('/auth/verify-code') && method === 'POST')
      return json(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600, onboarding_required: false });

    if (path.endsWith('/auth/onboarding') && method === 'POST')
      return json(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600 });

    if (path.endsWith('/auth/refresh') && method === 'POST')
      return json(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600 });

    if (path.endsWith('/operator-session'))
      return json(route, { authenticated: true, employee: EMPLOYEES[0], expires_at: '2099-12-31T23:59:59Z' });

    await route.fulfill({ status: 404, body: JSON.stringify({ code: 'NOT_FOUND', path }), contentType: 'application/json' });
  });
}

export type FiscalOverrides = {
  finalize?: unknown | number;
  finalizeState?: 'FISCALIZED' | 'FAILED' | 'UNKNOWN';
  saleOnOpen?: unknown;
  operation?: unknown;
};

let fiscalSaleCounter = 0;
let lastSale: Record<string, unknown> = {};

/**
 * Mocks the fiscal backend (http://localhost:8080).
 * This represents the Datecs/Daisy device boundary — the last software layer
 * before physical device protocol execution.
 */
export async function setupFiscalRoutes(page: Page, overrides: FiscalOverrides = {}) {
  fiscalSaleCounter = 0;
  lastSale = {};

  // pingFiscal calls HEAD /connectivity/ping
  await page.route(FISCAL_PING, (route) => {
    if (route.request().method() === 'HEAD')
      return route.fulfill({ status: 204, headers: { 'X-BeeFiscal-Ping': '1' } });
    return route.continue();
  });

  await page.route(`${FISCAL_API}/**`, async (route) => {
    const url = route.request().url();
    const method = route.request().method();
    const path = new URL(url).pathname;

    if (path.endsWith('/clock-sync') && method === 'POST')
      return json(route, { state: 'VERIFIED' });

    if (path.endsWith('/readiness:refresh') && method === 'POST')
      return json(route, { ready: true });

    if (path.endsWith('/sessions') && method === 'POST')
      return json(route, WORKSTATION_SESSION, 201);

    if (path.endsWith('/sales') && method === 'GET') {
      const items = lastSale?.state === 'OPEN' ? [lastSale] : [];
      return json(route, { items, page: { has_more: false } });
    }

    if (path.endsWith('/sales:open-with-line') && method === 'POST') {
      fiscalSaleCounter++;
      const body = JSON.parse(route.request().postData() || '{}');
      lastSale = {
        ...makeFiscalSale({
          sale_id: `sale-${fiscalSaleCounter}`,
          external_id: body.client_sale_surrogate_id,
          version: 1,
          lines: [{ ...body.line }],
          totals: { gross: body.line.unit_price },
        }),
        ...(overrides.saleOnOpen ?? {}),
      };
      return json(route, lastSale, 201);
    }

    if (/\/sales\/[^/]+\/lines$/.test(path) && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const existing = ((lastSale.lines ?? []) as unknown[]);
      const existingLine = existing.find((l: any) => l.line_id === body.line_id);
      if (!existingLine) (lastSale.lines as unknown[]).push(body);
      lastSale = { ...lastSale, version: Number(lastSale.version) + 1 };
      return json(route, lastSale, 201);
    }

    if (/\/sales\/[^/]+\/lines\/[^/]+$/.test(path) && method === 'PATCH') {
      const body = JSON.parse(route.request().postData() || '{}');
      lastSale = {
        ...lastSale,
        version: Number(lastSale.version) + 1,
        lines: [body],
        totals: { gross: body.unit_price },
      };
      return json(route, lastSale);
    }

    if (/\/sales\/[^/]+\/lines\/[^/]+:cancel$/.test(path) && method === 'POST') {
      lastSale = { ...lastSale, version: Number(lastSale.version) + 1, lines: [], totals: { gross: { amount: '0.00', currency: 'EUR' } } };
      return json(route, lastSale);
    }

    if (/\/sales\/[^/]+:finalize$/.test(path) && method === 'POST') {
      if (typeof overrides.finalize === 'number')
        return json(route, { code: 'ERROR' }, overrides.finalize);
      const state = overrides.finalizeState ?? 'FISCALIZED';
      if (state === 'FISCALIZED')
        lastSale = { ...lastSale, state: 'COMPLETED', version: Number(lastSale.version) + 1 };
      return json(route, overrides.finalize ?? {
        operation_id: `fiscal-${fiscalSaleCounter}`,
        type: 'FISCAL_SALE',
        state,
        fiscal_reference: state === 'FISCALIZED' ? `FD-E2E-${fiscalSaleCounter}` : null,
        allowed_actions: state === 'UNKNOWN' ? ['RECONCILE', 'READ'] : [],
      }, 202);
    }

    if (/\/sales\/[^/]+$/.test(path) && method === 'GET')
      return json(route, lastSale);

    if (/\/operations\/[^/:]+$/.test(path) && method === 'GET') {
      const operationState = overrides.finalizeState ?? 'FISCALIZED';
      return json(route, overrides.operation ?? {
        operation_id: `fiscal-${fiscalSaleCounter}`,
        type: 'FISCAL_SALE',
        state: operationState,
        fiscal_reference: operationState === 'FISCALIZED' ? `FD-E2E-${fiscalSaleCounter}` : null,
        allowed_actions: operationState === 'UNKNOWN' ? ['RECONCILE', 'READ'] : [],
      });
    }

    if (/\/operations\/[^/:]+:reconcile$/.test(path) && method === 'POST') {
      lastSale = { ...lastSale, state: 'COMPLETED', version: Number(lastSale.version) + 1 };
      return json(route, {
        operation_id: `fiscal-${fiscalSaleCounter}`,
        type: 'FISCAL_SALE',
        state: 'FISCALIZED',
        fiscal_reference: `FD-E2E-${fiscalSaleCounter}`,
        allowed_actions: [],
      }, 202);
    }

    if (/\/registers\/[^/]+\/ble-sessions$/.test(path) && method === 'POST')
      return json(route, {
        ble_session_id: 'ble-session-1',
        edge_id: 'edge-1',
        device_id: 'device-1',
        location_id: 'loc-1',
        binding_version: 1,
        expires_at: '2099-12-31T23:59:59Z',
        public_key: '',
      });

    if (/\/registers\/[^/]+$/.test(path) && method === 'GET')
      return json(route, { id: REGISTER_ID, type: 'FISCAL_REGISTER', device_id: 'device-1' });

    if (/\/ble-sessions\/[^/]+\/revoke$/.test(path) && method === 'POST')
      return json(route, {});

    if (path.endsWith('/workstations') || path.includes('/workstations/'))
      return json(route, {});

    await route.fulfill({ status: 404, body: JSON.stringify({ code: 'NOT_FOUND', path }), contentType: 'application/json' });
  });
}

// ── Auth bypass ──────────────────────────────────────────────────────────────

/**
 * Pre-populates AsyncStorage (localStorage in web) with tokens so the app
 * auto-authenticates without going through the email OTP screen.
 * Call before page.goto().
 */
export async function authenticate(page: Page, token = TEST_JWT) {
  await page.addInitScript(
    ([tok, ref]) => {
      localStorage.setItem('minipos-access-token', tok);
      localStorage.setItem('minipos-refresh-token', ref);
    },
    [token, TEST_REFRESH] as const,
  );
}

/**
 * Full setup: route mocks + auth bypass. Call before page.goto().
 */
export async function setup(
  page: Page,
  overrides: { minipos?: MiniposOverrides; fiscal?: FiscalOverrides } = {},
) {
  await setupMiniposRoutes(page, overrides.minipos);
  await setupFiscalRoutes(page, overrides.fiscal);
  await authenticate(page);
}

/**
 * Waits for the app to finish startup and show the product catalog.
 */
export async function waitForReady(page: Page, text = /Готово|Смяната е отворена|Отворената смяна е възстановена/) {
  await page.getByTestId('status-transport').filter({ hasText: text }).waitFor({ timeout: 15_000 });
}

/**
 * Completes the email auth flow via the UI (for auth-specific tests).
 */
export async function loginViaUI(page: Page, email = 'operator@example.com', code = '123456') {
  await page.getByTestId('operator-login').waitFor({ timeout: 5_000 });
  await page.locator('input[type="email"], [placeholder="email@example.com"]').fill(email);
  await page.getByTestId('operator-login-start').click();
  await page.locator('input[keyboardtype="number-pad"], input[placeholder="000000"]').fill(code);
  await page.getByTestId('operator-login-verify').click();
}
