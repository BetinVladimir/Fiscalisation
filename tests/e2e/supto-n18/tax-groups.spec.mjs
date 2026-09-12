import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../..');
const read = (path) => readFileSync(resolve(root, path), 'utf8');

test('BG universal profile exposes every Narедба N-18 article 27 tax group', () => {
  const policy = read('fiscal-backend/internal/domain/policy.go');
  const journey = read('tests/e2e/full-fiscal/run.sh');
  const expected = { A: '0.00', B: '20.00', C: '20.00', D: '9.00' };
  for (const [code, rate] of Object.entries(expected)) {
    assert.match(policy, new RegExp(`Code: "${code}", Rate: "${rate.replace('.', '\\.')}`));
    assert.match(journey, new RegExp(`sale_flow CASH tax-${code.toLowerCase()} ${code}`));
  }
  assert.match(journey, /map\(\.code\) == \["A","B","C","D"\]/);
});
