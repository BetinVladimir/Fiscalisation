import assert from 'node:assert/strict';
import test from 'node:test';
import { auditFilterDatesValid } from './auditFilters.ts';

test('audit optional bounds and equal instants across offsets', () => {
  assert.equal(auditFilterDatesValid({ from: '', to: '' }), true);
  assert.equal(auditFilterDatesValid({ from: '2024-02-29T12:00:00+03:00', to: '2024-02-29T09:00:00Z' }), true);
  assert.equal(auditFilterDatesValid({ from: ' 2026-10-12T00:00:00Z ', to: '' }), true);
});
test('audit rejects impossible dates, reversed instants and incomplete timestamps', () => {
  for (const from of ['2026-02-29T00:00:00Z', '2024-02-30T00:00:00Z', '2026-13-01T00:00:00Z', '2026-10-12', '2026-10-12T24:00:00Z', '2026-10-12T00:00:00+24:00']) {
    assert.equal(auditFilterDatesValid({ from, to: '' }), false, from);
  }
  assert.equal(auditFilterDatesValid({ from: '2026-10-12T00:00:01Z', to: '2026-10-12T00:00:00Z' }), false);
});
