import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../..');
const trace = JSON.parse(readFileSync(resolve(root, 'contracts/supto-annex29-trace.json'), 'utf8'));
const blocked = trace.requirements.filter((r) => r.production_blocked || !['PASS', 'NOT_APPLICABLE'].includes(r.status));

if (blocked.length) {
  console.error('SUPTO certification gate: BLOCKED');
  for (const item of blocked) console.error(`- ${item.id} [${item.status}]: ${item.gap || 'missing PASS evidence'}`);
  process.exit(1);
}

console.log('SUPTO certification gate: PASS');
