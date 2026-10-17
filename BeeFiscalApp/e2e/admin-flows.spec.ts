import { expect, test } from '@playwright/test';
import { DEVICE_ID, LOCATION_ID, OPERATOR_ID, REGISTER_ID, SECOND_DEVICE_ID, SECOND_REGISTER_ID, mockFiscalApi, openApp } from './helpers';

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
  await page.getByTestId('smart-device-activation-code').fill('bc1234');
  await page.getByTestId('smart-device-activation-lookup').click();
  await expect(page.getByTestId('smart-device-activation-preview')).toContainText('BC-001');
  await page.getByTestId('bluecash-activate').click();
  await expect(page.getByTestId('bluecash-activation-result')).toContainText('ACTIVE');
  const confirmed = mutations.find(x => x.path === '/device-activation-requests/activation-1:confirm')!;
  expect(confirmed.body).toEqual({ user_code: 'BC1234', location_id: LOCATION_ID, register_id: REGISTER_ID, roles: ['PAYMENT_TERMINAL'] });
});

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    test(`Paper device list and actions fit ${size}/${mode}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      await openApp(page);
      await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toContainText('Datecs counter');
      await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toContainText('ACTIVE');
      const device = page.getByTestId(`device.item.${DEVICE_ID}`);
      const affordance = await device.evaluate(element => {
        const style = getComputedStyle(element);
        return { border: parseFloat(style.borderTopWidth), borderStyle: style.borderTopStyle,
          color: style.borderTopColor, height: element.getBoundingClientRect().height };
      });
      expect(affordance.border).toBeGreaterThanOrEqual(1);
      expect(affordance.borderStyle).toBe('solid');
      expect(affordance.color).not.toBe('rgba(0, 0, 0, 0)');
      expect(affordance.height).toBeGreaterThanOrEqual(48);
      await device.focus();
      await page.keyboard.press('Enter');
      await expect(page.getByTestId('device-readiness')).toBeVisible();
      await expect(page.getByTestId('device-diagnostics')).toBeEnabled();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}

test('administration keeps edited location input after API error and retries with the same payload', async ({ page }) => {
  await mockFiscalApi(page);
  let attempts = 0;
  const bodies: Array<Record<string, unknown>> = [];
  await page.route(`**/locations/${LOCATION_ID}`, route => {
    if (route.request().method() !== 'PATCH') return route.fallback();
    attempts += 1;
    bodies.push(JSON.parse(route.request().postData() ?? '{}'));
    if (attempts === 1) return route.fulfill({ status: 503, contentType: 'application/json', body: '{"code":"UNAVAILABLE"}' });
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: LOCATION_ID, code: 'SOF-NEW', name: 'Sofia', version: 2 }) });
  });
  await openApp(page);
  await page.getByTestId('tab-Администриране').click();
  const code = page.getByTestId('admin-location-code');
  await code.fill('SOF-NEW');
  await page.getByTestId('admin-location-save').click();
  await expect(page.getByTestId('status-transport')).toContainText('UNAVAILABLE');
  await expect(code).toHaveValue('SOF-NEW');
  await page.getByTestId('admin-location-save').click();
  await expect(page.getByTestId('status-transport')).toContainText('готово');
  expect(bodies).toHaveLength(2);
  expect(bodies[0].code).toBe('SOF-NEW');
  expect(bodies[1].code).toBe('SOF-NEW');
});

test('administration rejects invalid editable transport JSON before device save', async ({ page }) => {
  await mockFiscalApi(page);
  await openApp(page);
  await page.getByTestId('tab-Администриране').click();
  const parameters = page.getByTestId('admin-device-transport-parameters');
  const save = page.getByTestId('admin-device-save');
  await parameters.fill('{invalid');
  await expect(page.getByText('Въведете валиден JSON обект.')).toBeVisible();
  await expect(save).toBeDisabled();
  await parameters.fill('{"baud":115200}');
  await expect(save).toBeEnabled();
  await expect(page.getByText('Разширени транспортни параметри в JSON формат.')).toBeVisible();
});

test('disconnected device displays its real status', async ({ page }) => {
  await mockFiscalApi(page, { devicesDisconnected: true });
  await openApp(page);
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toContainText('DISCONNECTED');
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).not.toContainText('READY');
});

test('active registration does not present an unavailable fiscal device as ready', async ({ page }) => {
  await mockFiscalApi(page, { deviceReady: false });
  await openApp(page);
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toContainText('ACTIVE');
  await expect(page.getByTestId('device-readiness')).toContainText('Не е готово');
  await expect(page.getByTestId('device-readiness')).not.toContainText('Готово');
});

test('missing adapter health does not suppress fiscal readiness', async ({ page }) => {
  await mockFiscalApi(page);
  await page.route(`**/devices/${DEVICE_ID}/health`, route => route.fulfill({ status: 404, contentType: 'application/json', body: '{"code":"DEVICE_HEALTH_NOT_FOUND"}' }));
  await openApp(page);
  await expect(page.getByTestId('device-readiness')).toHaveText('Готовност: Готово');
  await expect(page.getByTestId('device-realtime-health')).toContainText('UNKNOWN');
});

test('Paper device search filters records and distinguishes no matches from empty API', async ({ page }) => {
  await mockFiscalApi(page);
  await openApp(page);
  const search = page.getByTestId('devices.search');
  await search.fill('unknown printer');
  await expect(page.getByTestId('devices.no-matches')).toBeVisible();
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toHaveCount(0);
  await expect(page.getByTestId('devices.empty')).toHaveCount(0);
  await search.fill('Datecs');
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeVisible();
});

test('register selection reloads operations and reports for the selected register', async ({ page }) => {
  const requested: string[] = [];
  page.on('request', request => {
    if (request.method() === 'GET' && /\/(operations|reports)\?/.test(request.url())) requested.push(request.url());
  });
  await mockFiscalApi(page, { secondRegister: true });
  await openApp(page);
  await page.getByTestId('tab-Операции').click();
  await expect(page.getByTestId('operations.screen')).toBeVisible();
  await page.getByRole('radio', { name: 'R02', exact: true }).click();
  await expect.poll(() => requested.some(url => url.includes(`/operations?register_id=${SECOND_REGISTER_ID}`))).toBe(true);
  await page.getByTestId('tab-Отчети').click();
  await expect.poll(() => requested.some(url => url.includes(`/reports?register_id=${SECOND_REGISTER_ID}`))).toBe(true);
  await expect(page.getByRole('radio', { name: 'R02', exact: true })).toBeChecked();
});

test('device list follows backend register bindings when the register changes', async ({ page }) => {
  await mockFiscalApi(page, { secondRegister: true, secondDeviceReady: false });
  await openApp(page);
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeVisible();
  await expect(page.getByTestId('device-readiness')).toHaveText('Готовност: Готово');
  await expect(page.getByTestId(`device.item.${SECOND_DEVICE_ID}`)).toHaveCount(0);
  await page.getByRole('radio', { name: 'R02', exact: true }).first().click();
  await expect(page.getByTestId(`device.item.${SECOND_DEVICE_ID}`)).toBeVisible();
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toHaveCount(0);
  await expect(page.getByTestId('device-readiness')).toHaveText('Готовност: Не е готово');
  await page.getByRole('radio', { name: 'R01', exact: true }).first().click();
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeVisible();
  await expect(page.getByTestId(`device.item.${SECOND_DEVICE_ID}`)).toHaveCount(0);
  await expect(page.getByTestId('device-readiness')).toHaveText('Готовност: Готово');
});

test('device list shows an empty state with unavailable device actions', async ({ page }) => {
  await mockFiscalApi(page, { devicesEmpty: true });
  await page.goto('/');
  await expect(page.getByTestId('devices.empty')).toBeVisible();
  await expect(page.getByTestId('device-diagnostics')).toBeDisabled();
});

test('device refresh failure clears stale selection and retry restores API records', async ({ page }) => {
  await mockFiscalApi(page);
  await openApp(page);
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeVisible();
  let first = true;
  await page.route('**/devices?*', route => {
    if (first) {
      first = false;
      return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'UNAVAILABLE' }) });
    }
    return route.fallback();
  });
  await page.getByRole('button', { name: 'Обнови Устройства' }).click();
  await expect(page.getByTestId('devices.error')).toBeVisible();
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toHaveCount(0);
  await expect(page.getByTestId('device-control-plane')).toHaveCount(0);
  await page.getByRole('button', { name: 'Опитай отново' }).click();
  await expect(page.getByTestId(`device.item.${DEVICE_ID}`)).toBeVisible();
  await expect(page.getByTestId('devices.error')).toHaveCount(0);
});

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    test(`Paper administration cards fit ${size}/${mode}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Администриране').click();
      await expect(page.getByTestId('admin-editors')).toBeVisible();
      await expect(page.getByTestId('admin-location-code')).toBeVisible();
      await expect(page.getByTestId('admin-location-save')).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}

test('UNKNOWN operation is reconciled once and never re-submitted as a fiscal sale', async ({ page }) => {
  const mutations = await mockFiscalApi(page);
  await openApp(page);
  await page.getByTestId('tab-Операции').click();
  await expect(page.getByTestId('operation-unknown')).toBeVisible();
  await page.getByTestId('operation-reconcile-operation-unknown-1').click();
  await expect.poll(() => mutations.filter(x => x.path.endsWith('/reconcile')).length).toBe(1);
  expect(mutations.some(x => /\/sales/.test(x.path))).toBe(false);
});

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    test(`Paper operations list fits ${size}/${mode}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Операции').click();
      await expect(page.getByTestId('operations.screen')).toBeVisible();
      await expect(page.getByTestId('operation-unknown')).toContainText('FISCAL_SALE');
      await expect(page.getByTestId('operation-reconcile-operation-unknown-1')).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
    });
  }
}

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    test(`Paper reports list fits ${size}/${mode}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Отчети').click();
      await expect(page.getByTestId('reports.screen')).toBeVisible();
      await expect(page.getByTestId('reports.item.report-1')).toContainText('X');
      await expect(page.getByTestId('reports.create.Z')).toBeEnabled();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
      const report = page.getByTestId('reports.item.report-1');
      await report.scrollIntoViewIfNeeded();
      const reportBox = await report.boundingBox();
      const footerBox = await page.getByTestId('status-transport').boundingBox();
      expect(reportBox && footerBox && reportBox.y + reportBox.height <= footerBox.y + 1).toBe(true);
    });
  }
}

test('report command stays disabled while request is pending and uses one idempotent POST', async ({ page }) => {
  await mockFiscalApi(page);
  let release!: () => void;
  const pending = new Promise<void>(resolve => { release = resolve; });
  const posts: Array<{ key?: string; body: unknown }> = [];
  await page.route('**/registers/*/reports', async route => {
    if (route.request().method() !== 'POST') return route.fallback();
    posts.push({ key: route.request().headers()['idempotency-key'], body: JSON.parse(route.request().postData() ?? '{}') });
    await pending;
    return route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify({ id: 'report-z-1', state: 'COMPLETED', fiscal_reference: 'Z-REF-1' }) });
  });
  await openApp(page);
  await page.getByTestId('tab-Отчети').click();
  const create = page.getByTestId('reports.create.Z');
  await create.click();
  await expect.poll(() => posts.length).toBe(1);
  await expect(create).toBeDisabled();
  release();
  await expect(create).toBeEnabled();
  expect(posts).toHaveLength(1);
  expect(posts[0].key).toBeTruthy();
  expect(posts[0].body).toEqual({ type: 'Z' });
});

test('reports show a true empty state without stale devices', async ({ page }) => {
  await mockFiscalApi(page, { reportsEmpty: true });
  await openApp(page);
  await page.getByTestId('tab-Отчети').click();
  await expect(page.getByTestId('reports.empty')).toBeVisible();
  await expect(page.getByTestId('reports.item.report-1')).toHaveCount(0);
  await expect(page.getByText('Datecs counter')).toHaveCount(0);
});

test('reports API error does not display stale records and retry recovers', async ({ page }) => {
  await mockFiscalApi(page);
  let first = true;
  await page.route('**/reports*', route => {
    if (route.request().method() !== 'GET') return route.fallback();
    if (first) {
      first = false;
      return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'UNAVAILABLE' }) });
    }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [{ id: 'report-1', type: 'X', state: 'COMPLETED', fiscal_reference: 'X-REF-1' }], page: { has_more: false } }) });
  });
  await openApp(page);
  await page.getByTestId('tab-Отчети').click();
  await expect(page.getByTestId('reports.error')).toBeVisible();
  await expect(page.getByTestId('reports.item.report-1')).toHaveCount(0);
  await expect(page.getByTestId('reports.empty')).toHaveCount(0);
  await page.getByRole('button', { name: 'Опитай отново' }).click();
  await expect(page.getByTestId('reports.item.report-1')).toBeVisible();
  await expect(page.getByTestId('reports.error')).toHaveCount(0);
});

test('report command and audit filters reach the API without client-side fabricated results', async ({ page }) => {
  const mutations = await mockFiscalApi(page);
  await openApp(page);
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

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    test(`Paper audit journal fits ${size}/${mode}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Одит').click();
      await expect(page.getByTestId('audit-filters')).toBeVisible();
      await expect(page.getByTestId('audit-event-audit-1')).toContainText('SALE_FINALIZED');
      await expect(page.getByTestId('audit.details-content.audit-1')).toHaveCount(0);
      const detailsAction=page.getByTestId('audit.details.audit-1');
      await expect(detailsAction).toHaveAttribute('aria-expanded','false');
      expect(await detailsAction.evaluate(node=>parseFloat(getComputedStyle(node).borderTopWidth))).toBeGreaterThanOrEqual(1);
      expect((await detailsAction.boundingBox())!.height).toBeGreaterThanOrEqual(48);
      await detailsAction.focus();await page.keyboard.press('Enter');
      await expect(detailsAction).toHaveAttribute('aria-expanded','true');
      await expect(page.getByTestId('audit.details-content.audit-1')).toContainText('Hash: abc123');
      await page.getByTestId('audit.details-content.audit-1').scrollIntoViewIfNeeded();
      const detailsBox = await page.getByTestId('audit.details-content.audit-1').boundingBox();
      const detailsFooter = await page.getByTestId('status-transport').boundingBox();
      expect(detailsBox && detailsFooter && detailsBox.y + detailsBox.height <= detailsFooter.y + 1).toBe(true);
      await detailsAction.focus();await page.keyboard.press('Space');
      await expect(detailsAction).toHaveAttribute('aria-expanded','false');
      await expect(page.getByTestId('audit.details-content.audit-1')).toHaveCount(0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
      const event = page.getByTestId('audit-event-audit-1');
      await event.scrollIntoViewIfNeeded();
      const eventBox = await event.boundingBox();
      const footerBox = await page.getByTestId('status-transport').boundingBox();
      expect(eventBox && footerBox && eventBox.y + eventBox.height <= footerBox.y + 1).toBe(true);
    });
  }
}

test('audit journal shows empty state without stale device data', async ({ page }) => {
  await mockFiscalApi(page, { auditEmpty: true });
  await openApp(page);
  await page.getByTestId('tab-Одит').click();
  await expect(page.getByTestId('audit.empty')).toBeVisible();
  await expect(page.getByText('Datecs counter')).toHaveCount(0);
});

test('audit API failure shows an error and retry recovers the read-only list', async ({ page }) => {
  await mockFiscalApi(page);
  let first = true;
  await page.route('**/audit-events*', route => {
    if (first) {
      first = false;
      return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'UNAVAILABLE' }) });
    }
    return route.fallback();
  });
  await openApp(page);
  await page.getByTestId('tab-Одит').click();
  await expect(page.getByTestId('audit.error')).toBeVisible();
  await expect(page.getByTestId('audit-event-audit-1')).toHaveCount(0);
  await page.getByRole('button', { name: 'Опитай отново' }).click();
  await expect(page.getByTestId('audit-event-audit-1')).toBeVisible();
  await expect(page.getByTestId('audit.error')).toHaveCount(0);
});

for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const mode of ['light', 'dark'] as const) {
    test(`Paper regulatory settings fit ${size}/${mode}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: mode, reducedMotion: 'reduce' });
      await mockFiscalApi(page);
      await openApp(page);
      if (size === 'xs') await page.getByTestId('nav.open').click();
      await page.getByTestId('tab-Настройки').click();
      await expect(page.getByTestId('settings.policy')).toContainText('Официална валута: EUR');
      await expect(page.getByTestId('settings.tax-group.B')).toContainText('20.00%');
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
      const group = page.getByTestId('settings.tax-group.B');
      await group.scrollIntoViewIfNeeded();
      const groupBox = await group.boundingBox();
      const footerBox = await page.getByTestId('status-transport').boundingBox();
      expect(groupBox && footerBox && groupBox.y + groupBox.height <= footerBox.y + 1).toBe(true);
    });
  }
}

test('regulatory settings show an empty tax-group state', async ({ page }) => {
  await mockFiscalApi(page, { taxGroupsEmpty: true });
  await openApp(page);
  await page.getByTestId('tab-Настройки').click();
  await expect(page.getByTestId('settings.tax-groups')).toContainText('Няма данъчни групи в API.');
});

test('regulatory policy API failure clears stale policy and retry recovers', async ({ page }) => {
  await mockFiscalApi(page);
  let first = true;
  await page.route('**/country-policy', route => {
    if (first) {
      first = false;
      return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'UNAVAILABLE' }) });
    }
    return route.fallback();
  });
  await openApp(page);
  await page.getByTestId('tab-Настройки').click();
  await expect(page.getByTestId('settings.error')).toBeVisible();
  await expect(page.getByTestId('settings.policy')).toHaveCount(0);
  await page.getByRole('button', { name: 'Опитай отново' }).click();
  await expect(page.getByTestId('settings.policy')).toContainText('Официална валута: EUR');
  await expect(page.getByTestId('settings.error')).toHaveCount(0);
});

test('operational shell exposes a bounded language list and persisted theme', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 });
  await mockFiscalApi(page);
  await openApp(page);
  await page.getByTestId('shell.language.open').click();
  const list = page.getByTestId('shell.language.list');
  await expect(list).toBeVisible();
  expect((await list.boundingBox())?.height).toBeLessThanOrEqual(225);
  await page.getByTestId('shell.language.ru').scrollIntoViewIfNeeded();
  const languageOption = page.getByTestId('shell.language.ru');
  expect(await languageOption.evaluate(element => parseFloat(getComputedStyle(element).borderTopWidth))).toBeGreaterThanOrEqual(1);
  expect((await languageOption.boundingBox())?.height).toBeGreaterThanOrEqual(48);
  await languageOption.click();
  await expect(page.getByTestId('shell.language.open')).toContainText('RU');
  await page.getByTestId('shell.theme.toggle').click();
  await page.getByTestId('shell.theme.toggle').click();
  const themeLabel = await page.getByTestId('shell.theme.toggle').getAttribute('aria-label');
  await page.reload();
  await expect(page.getByTestId('shell.language.open')).toContainText('RU');
  await expect(page.getByTestId('shell.theme.toggle')).toHaveAttribute('aria-label', themeLabel ?? '');
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
});

test('backend failure is fail-closed and exposes CORE OFFLINE instead of stale device controls', async ({ page }) => {
  await mockFiscalApi(page, { devicesStatus: 503 });
  await page.goto('/');
  await expect(page.getByTestId('status-fiscal-device')).toContainText('CORE OFFLINE');
  await expect(page.getByText(/API отказ/)).toBeVisible();
  await expect(page.getByTestId('devices.error')).toBeVisible();
  await expect(page.getByTestId('device-control-plane')).toHaveCount(0);
});
