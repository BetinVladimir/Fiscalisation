import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { setup, waitForPOS } from './helpers';
import { translations } from '../src/ui/workTranslations.generated';
import { collectContrastCandidates } from './contrast-candidates';

const ids = {
  awaiting: '00000000-0000-4000-8000-000000000001',
  unknown: '00000000-0000-4000-8000-000000000002',
  pending: '00000000-0000-4000-8000-000000000003',
  synced: '00000000-0000-4000-8000-000000000004',
  failed: '00000000-0000-4000-8000-000000000005',
  fiscalized: '00000000-0000-4000-8000-000000000006',
};

async function seedOutbox(page: import('@playwright/test').Page) {
  await page.evaluate(async ids => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open('beeloy-miniposweb', 1);
      request.onupgradeneeded = () => request.result.createObjectStore('fiscal-outbox', { keyPath: 'intent.client_operation_id' });
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    const now = new Date().toISOString();
    const sync = {
      shift_id: 'shift-001',
      lines: [{ line_id: 'line-001', product_id: 'prod-001', name: 'Кафе', quantity: '1.000', unit_price: { amount: '2.50', currency: 'EUR' }, tax_group: 'B' }],
      payments: [{ payment_id: 'cash-001', type: 'CASH', amount: { amount: '2.50', currency: 'EUR' } }],
    };
    const records = [
      { id: ids.awaiting },
      { id: ids.unknown, result: { state: 'UNKNOWN' } },
      { id: ids.pending, result: { state: 'FISCALIZED' }, posSync: { ...sync, external_id: 'ext-003' } },
      { id: ids.synced, result: { state: 'FISCALIZED' }, posSync: { ...sync, external_id: 'ext-004', synced_at: now } },
      { id: ids.failed, result: { state: 'FAILED' } },
      { id: ids.fiscalized, result: { state: 'FISCALIZED' } },
    ];
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction('fiscal-outbox', 'readwrite');
      for (const record of records) tx.objectStore('fiscal-outbox').put({
        intent: {
          protocol_version: '1.0', intent_id: record.id, action: 'SALE_FINALIZE', client_operation_id: record.id,
          receipt_session_id: 'receipt-e2e', client_sale_surrogate_id: 'sale-e2e', server_sale_id: '',
          operator_code: '001', tenant_id: '00000000-0000-4000-8000-000000000001', location_id: 'loc-001', register_id: 'reg-001',
          edge_device_id: 'adapter-001', binding_generation: 1, command: 'SALE_FINALIZE',
          items: [{ item_id: 'line-001', name: 'Кафе', quantity: '1.000', unit_price: { amount: '2.50', currency: 'EUR' }, tax_group: 'B' }],
          payments: [{ payment_id: 'cash-001', type: 'CASH', amount: { amount: '2.50', currency: 'EUR' } }],
          metadata: {}, canonical_payload: {}, payload_sha256: 'e2e-fixture',
        }, result: record.result && { operation_id: record.id, state: record.result.state, certainty: record.result.state === 'UNKNOWN' ? 'UNKNOWN' : record.result.state === 'FAILED' ? 'PROVEN_FAILURE' : 'PROVEN_SUCCESS', updated_at: now }, posSync: record.posSync,
        createdAt: now, updatedAt: now,
      });
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
    db.close();
  }, ids);
}

async function clearOutbox(page: import('@playwright/test').Page) {
  await page.evaluate(async () => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open('beeloy-miniposweb', 1);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction('fiscal-outbox', 'readwrite');
      tx.objectStore('fiscal-outbox').clear();
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
    db.close();
  });
}

async function failOutboxReads(page: import('@playwright/test').Page) {
  await page.evaluate(() => {
    const original = IDBDatabase.prototype.transaction;
    Object.defineProperty(IDBDatabase.prototype, 'transaction', {
      configurable: true,
      value: function (this: IDBDatabase, storeNames: string | string[], mode?: IDBTransactionMode) {
        if (storeNames === 'fiscal-outbox' && (!mode || mode === 'readonly')) throw new DOMException('E2E IndexedDB read failure', 'UnknownError');
        return original.call(this, storeNames, mode);
      },
    });
  });
}

async function tabTo(page: import('@playwright/test').Page, testId: string) {
  for (let step = 0; step < 30; step++) {
    await page.keyboard.press('Tab');
    if (await page.getByTestId(testId).evaluate(element => element === document.activeElement || element.contains(document.activeElement))) return;
  }
  throw new Error(`Keyboard Tab could not reach ${testId}`);
}

test('keyboard reaches the offline queue, refresh and language menu', async ({ page }) => {
  await setup(page);
  await page.goto('/');
  await waitForPOS(page);
  await tabTo(page, 'app-outbox');
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('offline-outbox-empty')).toBeVisible();
  await tabTo(page, 'offline-outbox-refresh');
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('offline-outbox-empty')).toBeVisible();
  await tabTo(page, 'app-language-open');
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('app-language-ru')).toBeAttached();
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('app-language-open')).toBeFocused();
});

test('read-only offline queue shows empty, pending, uncertain and synced states', async ({ page }) => {
  await setup(page, { minipos: { importOfflineOrder: 503 } });
  await page.goto('/');
  await waitForPOS(page);
  await page.getByTestId('app-outbox').click();
  await expect(page.getByTestId('offline-outbox-empty')).toBeVisible();
  await seedOutbox(page);
  await page.getByTestId('offline-outbox-refresh').click();
  for (const [id, label] of [
    [ids.awaiting, translations.bg.outboxAwaiting],
    [ids.unknown, translations.bg.outboxUncertain],
    [ids.pending, translations.bg.outboxPendingSync],
    [ids.synced, translations.bg.outboxSynced],
    [ids.failed, translations.bg.outboxFailed],
    [ids.fiscalized, translations.bg.outboxFiscalized],
  ] as const) await expect(page.getByTestId(`offline-outbox-status-${id}`)).toHaveText(label);
  await page.getByTestId('app-language-open').click();
  await page.getByTestId('app-language-ru').scrollIntoViewIfNeeded();
  await page.getByTestId('app-language-ru').click();
  await expect(page.getByTestId(`offline-outbox-status-${ids.unknown}`)).toHaveText(translations.ru.outboxUncertain);
  await failOutboxReads(page);
  await page.getByTestId('offline-outbox-refresh').click();
  await expect(page.getByTestId('offline-outbox-error')).toHaveText(translations.ru.loadFailed);
  await expect(page.getByTestId(`offline-outbox-row-${ids.unknown}`)).toHaveCount(0);
  await page.reload();
  await waitForPOS(page);
  await page.getByTestId('app-outbox').click();
  await expect(page.getByTestId(`offline-outbox-row-${ids.unknown}`)).toBeVisible();
});

test('offline queue: empty, populated and load-error xs/md/lg × light/dark WCAG matrix', async ({ page }) => {
  test.setTimeout(120_000);
  await setup(page, { minipos: { importOfflineOrder: 503 } });
  const evidenceDir = join(process.cwd(), 'docs', 'ui-screenshots');
  await mkdir(evidenceDir, { recursive: true });
  const scans: {screen:string;viewport:string;theme:string;violations:Awaited<ReturnType<AxeBuilder['analyze']>>['violations'];incomplete:Awaited<ReturnType<AxeBuilder['analyze']>>['incomplete'];contrastCandidates:unknown[]}[] = [];
  for (const [size, width, height] of [['xs', 320, 800], ['md', 768, 900], ['lg', 1440, 1000]] as const) {
    await page.setViewportSize({ width, height });
    for (const theme of ['light', 'dark'] as const) {
      await page.addInitScript(value => localStorage.setItem('minipos-web-theme', value), theme);
      await page.goto('/');
      await waitForPOS(page);
      await clearOutbox(page);
      await page.getByTestId('app-outbox').click();
      await expect(page.getByTestId('offline-outbox-empty')).toBeVisible();
      await scan('empty');
      await seedOutbox(page);
      await page.getByTestId('offline-outbox-refresh').click();
      await expect(page.getByTestId(`offline-outbox-row-${ids.pending}`)).toBeVisible();
      await scan('populated');
      await failOutboxReads(page);
      await page.getByTestId('offline-outbox-refresh').click();
      await expect(page.getByTestId('offline-outbox-error')).toBeVisible();
      await scan('load-error');

      async function scan(state: string) {
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${state}/${size}/${theme}: overflow`).toBe(true);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze();
        const screenshot = await page.screenshot({ path: join(evidenceDir, `outbox-${state}-${size}-${theme}.png`), fullPage: true });
        scans.push({ screen: `outbox-${state}`, viewport: size, theme, violations: result.violations, incomplete: result.incomplete,
          contrastCandidates: await collectContrastCandidates(page, result, screenshot) });
      }
    }
  }
  expect(scans.flatMap(scan=>scan.incomplete),'Incomplete requires resolution').toEqual([]);
  await writeFile(join(process.cwd(), 'docs', 'ui-optimization-outbox-axe.json'), `${JSON.stringify({ scans }, null, 2)}\n`);
  expect(scans.flatMap(scan => scan.violations.map(violation => ({ viewport: scan.viewport, theme: scan.theme, id: violation.id })))).toEqual([]);
});

// Authored only; execution remains deferred by global forms/test gate.
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`foreign outbox ownership excluded ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await page.clock.install();await setup(page,{minipos:{importOfflineOrder:{id:'import-owned'}}});await page.goto('/');await waitForPOS(page);await seedOutbox(page);
  await page.evaluate(async id=>{
   const db=await new Promise<IDBDatabase>((resolve,reject)=>{const r=indexedDB.open('beeloy-miniposweb',1);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error);});
   const original=await new Promise<any>((resolve,reject)=>{const r=db.transaction('fiscal-outbox').objectStore('fiscal-outbox').get(id);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error);});
   await new Promise<void>((resolve,reject)=>{const tx=db.transaction('fiscal-outbox','readwrite');for(const [field,value] of [['tenant_id','foreign-tenant'],['register_id','foreign-register'],['operator_code','999']] as const){const key=`foreign-${field}`;tx.objectStore('fiscal-outbox').put({...original,intent:{...original.intent,client_operation_id:key,[field]:value},posSync:{...original.posSync,external_id:key}});}tx.oncomplete=()=>resolve();tx.onerror=()=>reject(tx.error);});db.close();
  },ids.pending);
  const imported:string[]=[];page.on('request',request=>{if(request.method()==='POST'&&request.url().includes('import-offline'))imported.push(JSON.stringify(request.postDataJSON()));});
  await page.getByTestId('app-outbox').click();await expect(page.getByTestId(`offline-outbox-row-${ids.pending}`)).toBeVisible();
  for(const field of ['tenant_id','register_id','operator_code'])await expect(page.getByTestId(`offline-outbox-row-foreign-${field}`)).toHaveCount(0);
  await page.clock.fastForward(15_000);await expect.poll(()=>imported.length).toBeGreaterThan(0);
  expect(imported.some(body=>body.includes('foreign-'))).toBe(false);
 });
}
