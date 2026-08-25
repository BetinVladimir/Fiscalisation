import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../..');
const read = (path) => readFileSync(resolve(root, path), 'utf8');
const trace = JSON.parse(read('contracts/supto-annex29-trace.json'));
const annex = read('docs/SUPTO/Приложение № 29 (RU).md');
const canonical = read('docs/SUPTO/Naredba N-18 2006 canonical clean RU ollama.md');
const runtime = read('contracts/openapi-runtime-v1.yaml');
const corrections = read('contracts/openapi-corrections-v1.yaml');
const fullJourney = read('tests/e2e/full-fiscal/run.sh');
const mobileJourney = read('minipos/BeeMiniPOS/e2e/web-interaction.mjs');

test('canonical N-18 and Annex 29 inputs are present and non-truncated', () => {
  assert.ok(canonical.length > 600_000, 'canonical N-18 translation appears truncated');
  assert.ok(annex.includes('## Требования к программному обеспечению'));
  for (let point = 1; point <= 23; point++) {
    assert.match(annex, new RegExp(`^${point}\\.`, 'm'), `Annex 29 point ${point} is absent`);
  }
});

test('every Annex requirement has a machine-readable disposition and evidence owner', () => {
  const expected = Array.from({ length: 24 }, (_, i) => `SUPTO-29-${String(i + 1).padStart(2, '0')}`);
  assert.deepEqual(trace.requirements.map((r) => r.id), expected);
  for (const req of trace.requirements) {
    assert.ok(['PASS', 'PARTIAL', 'NOT_APPLICABLE', 'EXTERNAL_BLOCKED'].includes(req.status), `${req.id}: invalid status`);
    assert.ok(req.owner, `${req.id}: missing owner`);
    assert.ok(Array.isArray(req.invariants) && req.invariants.length, `${req.id}: missing invariants`);
    if (req.status === 'PARTIAL' || req.status === 'EXTERNAL_BLOCKED') {
      assert.equal(req.production_blocked, true, `${req.id}: incomplete evidence must block certification`);
      assert.ok(req.gap, `${req.id}: incomplete evidence must describe the gap`);
    }
  }
});

test('all referenced automated evidence files exist', () => {
  for (const req of trace.requirements) {
    for (const field of ['unit_tests', 'integration_tests', 'e2e_tests', 'evidence']) {
      for (const path of req[field] || []) assert.ok(existsSync(resolve(root, path)), `${req.id}: missing ${field} file ${path}`);
    }
  }
});

test('runtime contract exposes mandatory sale lifecycle and workstation controls', () => {
  for (const path of [
    '/sales:open-with-line:',
    '/sales/{sale_id}/lines/{line_id}:cancel:',
    '/sales/{sale_id}:cancel:',
    '/sales/{sale_id}:reverse:',
    '/workstations/{workstation_id}/clock-sync:',
    '/workstations/{workstation_id}/sessions:',
    '/exports/periodized:',
  ]) assert.ok(runtime.includes(path), `missing OpenAPI path ${path}`);
  assert.ok(corrections.includes("$.paths['/audit-events'].get.parameters"), 'missing audit-events OpenAPI contract');
});

test('full journey covers onboarding, binding, sales, split tenders, receipt, storno and device protocol', () => {
  const evidence = [
    ['onboarding', '/auth/onboarding'],
    ['fiscal enrollment', '/fiscal-enrollment:verify'],
    ['operator synchronization', 'operator_ready'],
    ['cash sale', 'sale_flow CASH'],
    ['card sale', 'sale_flow CARD'],
    ['receipt readback', '/receipt'],
    ['storno', ':reverse'],
    ['device readiness protocol', '/readyz'],
    ['device intent protocol', '/intents'],
    ['device result protocol', '/operations/'],
    ['idempotent retry', 'Idempotency replay'],
  ];
  for (const [name, marker] of evidence) assert.ok(fullJourney.includes(marker), `full E2E misses ${name}`);
  assert.ok(mobileJourney.includes('touch split tender must preserve exact ordered EUR amounts'));
  assert.ok(mobileJourney.includes('exactly one aggregate finalize per checkout'));
});

test('UNP is generated at first line and passed through fiscal completion/reversal contracts', () => {
  assert.ok(runtime.includes('Атомарно открыть продажу первой строкой и выдать УНП'));
  assert.match(runtime, /regulatory_identifiers/);
  assert.match(fullJourney, /original_fiscal_reference/);
});

test('certification cannot silently pass while external or HIL evidence is incomplete', () => {
  const blocked = trace.requirements.filter((r) => r.production_blocked);
  assert.ok(blocked.length > 0, 'remove this guard only after every external/HIL artifact is accepted');
  for (const req of blocked) assert.ok(req.gap && req.status !== 'PASS', `${req.id}: inconsistent blocker`);
});
