import type { Page, Route } from '@playwright/test';

export const TEST_JWT =
  'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9' +
  '.eyJ0ZW5hbnRfaWQiOiIwMDAwMDAwMC0wMDAwLTQwMDAtODAwMC0wMDAwMDAwMDAwMDEiLCJzdWIiOiJ1c2VyLTEiLCJleHAiOjk5OTk5OTk5OTl9' +
  '.e2e_test';

export const TEST_REFRESH = 'refresh-e2e-test';

// Datecs/Daisy boundary: edge-agent local HTTP API
export const ADAPTER_ORIGIN = 'http://localhost:19001';
export const ADAPTER_BASE_URL = `${ADAPTER_ORIGIN}/beeloy/local/v1`;

// ── canonical mock data ────────────────────────────────────────────────────

export const EMPLOYEE = {
  id: 'emp-001',
  first_name: 'Иван',
  last_name: 'Иванов',
  operator_code: '001',
  roles: ['OPERATOR'],
};

export const SESSION = {
  authenticated: true,
  employee: EMPLOYEE,
  expires_at: new Date(Date.now() + 8 * 3600_000).toISOString(),
};

export const CONFIG = {
  location_id: 'loc-001',
  fiscal_register_id: 'reg-001',
  fiscal_adapter_id: 'adapter-001',
  binding_generation: 1,
  adapter_base_url: ADAPTER_BASE_URL,
};

export const PRODUCTS = [
  {
    id: 'prod-001',
    name: 'Кафе',
    price: { amount: '2.50', currency: 'EUR' },
    tax_group: 'B',
    status: 'ACTIVE',
  },
  {
    id: 'prod-002',
    name: 'Вода',
    price: { amount: '1.00', currency: 'EUR' },
    tax_group: 'B',
    status: 'ACTIVE',
  },
];

export const SHIFT = {
  id: 'shift-001',
  register_id: 'reg-001',
  employee_id: 'emp-001',
  state: 'OPEN',
  version: 1,
};

export const ORDER = {
  id: 'order-001',
  external_id: 'ext-001',
  version: 1,
};

export const LOCAL_TOKEN = {
  access_token: 'local-adapter-token',
  expires_at: new Date(Date.now() + 30 * 60_000).toISOString(),
  adapter_base_url: ADAPTER_BASE_URL,
};

// ── response helpers ───────────────────────────────────────────────────────

function respond(route: Route, value: object | number) {
  if (typeof value === 'number') {
    return route.fulfill({
      status: value,
      body: JSON.stringify({ code: 'ERROR' }),
      contentType: 'application/json',
    });
  }
  return route.fulfill({
    status: 200,
    body: JSON.stringify(value),
    contentType: 'application/json',
  });
}

// ── minipos-backend route mock ─────────────────────────────────────────────

export type MiniposOverrides = {
  session?: object | number;
  configuration?: object | number;
  products?: object | number;
  employees?: object | number;
  shifts?: object | number;
  openShift?: object | number;
  createOrder?: object | number;
  addLine?: object | number;
  checkoutBatch?: object | number;
  fiscalRouteHealth?: object | number;
  fiscalLocalTokens?: object | number;
  importOfflineOrder?: object | number;
  reverseOrder?: object | number;
  createProduct?: object | number;
  createEmployee?: object | number;
  requestCode?: number;
  verifyCode?: object | number;
};

export async function setupMiniposRoutes(page: Page, overrides: MiniposOverrides = {}) {
  await page.route('**/public/v1/minipos/**', async (route) => {
    const url = route.request().url();
    const method = route.request().method();

    if (url.includes('/auth/request-code'))
      return overrides.requestCode != null
        ? route.fulfill({ status: overrides.requestCode, body: '' })
        : route.fulfill({ status: 204, body: '' });

    if (url.includes('/auth/verify-code'))
      return respond(route, overrides.verifyCode ?? {
        access_token: TEST_JWT,
        refresh_token: TEST_REFRESH,
        expires_in: 3600,
        onboarding_required: false,
      });

    if (url.includes('/auth/onboarding'))
      return respond(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600 });

    if (url.includes('/auth/refresh'))
      return respond(route, { access_token: TEST_JWT, refresh_token: TEST_REFRESH, expires_in: 3600 });

    if (url.includes('/operator-session'))
      return respond(route, overrides.session ?? SESSION);

    if (url.includes('/configuration') && method === 'GET')
      return respond(route, overrides.configuration ?? CONFIG);

    if (url.includes('/fiscal-route-health'))
      return respond(route, overrides.fiscalRouteHealth ?? { status: 'ok' });

    if (url.includes('/fiscal-local-tokens'))
      return respond(route, overrides.fiscalLocalTokens ?? LOCAL_TOKEN);

    if (url.includes('/orders:import-offline') || url.includes('/orders%3Aimport-offline'))
      return respond(route, overrides.importOfflineOrder ?? { id: 'order-imported-001', state: 'ACCEPTED' });

    if (url.includes('/checkout-batch') && method === 'POST')
      return respond(route, overrides.checkoutBatch ?? {
        operation_id: 'op-cloud-001',
        state: 'FISCALIZED',
        fiscal_reference: 'CLOUD-REF-001',
        updated_at: new Date().toISOString(),
      });

    if (url.includes('/reversals') && method === 'POST')
      return respond(route, overrides.reverseOrder ?? {
        operation_id: 'op-rev-001',
        state: 'FISCALIZED',
        fiscal_reference: 'REV-001',
        updated_at: new Date().toISOString(),
      });

    if (url.includes('/lines') && method === 'POST')
      return respond(route, overrides.addLine ?? { id: 'line-001', version: 2 });

    if (url.includes('/shifts') && method === 'POST')
      return respond(route, overrides.openShift ?? SHIFT);

    if (url.includes('/shifts') && method === 'GET')
      return respond(route, overrides.shifts ?? { items: [SHIFT] });

    if (url.includes('/products') && method === 'POST')
      return respond(route, overrides.createProduct ?? {
        id: 'prod-new-001',
        name: 'Нов продукт',
        price: { amount: '5.00', currency: 'EUR' },
        tax_group: 'B',
        status: 'ACTIVE',
      });

    if (url.includes('/products') && method === 'GET')
      return respond(route, overrides.products ?? { items: PRODUCTS });

    if (url.includes('/employees') && method === 'POST')
      return respond(route, overrides.createEmployee ?? {
        id: 'emp-new-001',
        first_name: 'Петър',
        last_name: 'Петров',
        operator_code: '002',
        roles: ['OPERATOR'],
      });

    if (url.includes('/employees') && method === 'GET')
      return respond(route, overrides.employees ?? { items: [] });

    if (url.includes('/orders') && method === 'POST')
      return respond(route, overrides.createOrder ?? ORDER);

    await route.fulfill({ status: 404, body: JSON.stringify({ code: 'NOT_FOUND' }), contentType: 'application/json' });
  });
}

// ── Datecs/Daisy device boundary mock ─────────────────────────────────────

export type DeviceOverrides = {
  healthz?: object | number;
  readyz?: object | number;
  submit?: object | number;
  operation?: object | number;
  hangHealthz?: boolean;
};

export async function setupDeviceRoutes(page: Page, overrides: DeviceOverrides = {}) {
  await page.route(`${ADAPTER_ORIGIN}/**`, async (route) => {
    const url = route.request().url();
    const method = route.request().method();

    if (url.includes('/healthz')) {
      if (overrides.hangHealthz) return; // never respond → triggers AbortController timeout
      return respond(route, overrides.healthz ?? { status: 'ok', profile: 'edge-agent-s3' });
    }

    if (url.includes('/readyz'))
      return respond(route, overrides.readyz ?? { state: 'READY', profile: 'edge-agent-s3' });

    if (url.includes('/intents') && method === 'POST')
      return respond(route, overrides.submit ?? {
        operation_id: 'op-device-001',
        state: 'FISCALIZED',
        fiscal_reference: `DATECS-E2E-${Date.now()}`,
        updated_at: new Date().toISOString(),
      });

    if (url.includes('/operations/'))
      return respond(route, overrides.operation ?? {
        operation_id: 'op-device-001',
        state: 'FISCALIZED',
        fiscal_reference: `DATECS-E2E-${Date.now()}`,
        updated_at: new Date().toISOString(),
      });

    await route.fulfill({ status: 404, body: JSON.stringify({ code: 'NOT_FOUND' }), contentType: 'application/json' });
  });
}

// ── auth bypass ────────────────────────────────────────────────────────────

/**
 * Sets localStorage tokens before page load so the app skips the auth screen.
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
  overrides: { minipos?: MiniposOverrides; device?: DeviceOverrides } = {},
) {
  await setupMiniposRoutes(page, overrides.minipos);
  await setupDeviceRoutes(page, overrides.device);
  await authenticate(page);
}

/**
 * Waits for the product grid to appear (confirms main UI is ready).
 */
export async function waitForPOS(page: Page) {
  await page.getByText('Кафе').waitFor({ timeout: 10_000 });
}
