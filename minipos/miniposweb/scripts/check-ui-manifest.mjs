import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const root = join(import.meta.dirname, '..');
const manifest = JSON.parse(readFileSync(join(root, 'e2e', 'ui-manifest.json'), 'utf8'));
const requiredIds = [
  'auth/email', 'auth/otp', 'auth/onboarding', 'session/recovery',
  'shift/open', 'shift/close', 'sale/catalog', 'sale/cart', 'sale/payment',
  'sale/result', 'management/products', 'management/employees',
  'offline/outbox', 'settings/diagnostics', 'preferences/menus',
];
const issues = [];
if (manifest.application !== 'MiniPOS Web' || !Array.isArray(manifest.screens)) issues.push('Invalid root manifest');
const screens = manifest.screens || [];
const byId = new Map();
for (const screen of screens) {
  if (!screen.id || byId.has(screen.id)) issues.push(`Duplicate or missing ID: ${screen.id}`);
  byId.set(screen.id, screen);
  for (const key of ['source', 'entryAction', 'role', 'fixture', 'expectedScreenTestId']) {
    if (typeof screen[key] !== 'string' || !screen[key].trim()) issues.push(`${screen.id}: missing ${key}`);
  }
  if (!['verified', 'partial', 'open'].includes(screen.status)) issues.push(`${screen.id}: invalid status ${screen.status}`);
  if (!Array.isArray(screen.states) || !screen.states.length) issues.push(`${screen.id}: missing states`);
  if (!Array.isArray(screen.evidence)) issues.push(`${screen.id}: missing evidence array`);
  if (!Array.isArray(screen.openGates)) issues.push(`${screen.id}: missing openGates array`);
  if (screen.status === 'verified' && screen.openGates?.length) issues.push(`${screen.id}: verified with open gates`);
  if (screen.status !== 'verified' && !screen.openGates?.length) issues.push(`${screen.id}: incomplete without open gates`);
  if (screen.source && !existsSync(join(root, screen.source))) issues.push(`${screen.id}: source missing: ${screen.source}`);
  for (const path of screen.evidence || []) if (!existsSync(join(root, path))) issues.push(`${screen.id}: evidence missing: ${path}`);
}
for (const id of requiredIds) if (!byId.has(id)) issues.push(`Required state missing: ${id}`);
for (const screen of screens) if (!requiredIds.includes(screen.id)) issues.push(`Unreviewed state ID: ${screen.id}`);
for (const source of ['src/App.tsx', 'src/AuthFlow.tsx', 'src/Management.tsx', 'src/SalePanels.tsx', 'src/ui/PreferenceControls.tsx']) {
  const code = readFileSync(join(root, source), 'utf8');
  if (/<\s*(?:button|input|select)\b/.test(code)) issues.push(`${source}: native visual HTML control found`);
}
if (issues.length) {
  console.error(`UI manifest invalid (${issues.length} issue(s)):\n${issues.map(issue => `- ${issue}`).join('\n')}`);
  process.exit(2);
}
const incomplete = screens.filter(screen => screen.status !== 'verified');
console.log(`UI manifest structure valid: ${screens.length} states, ${screens.length - incomplete.length} verified, ${incomplete.length} incomplete`);
if (process.argv.includes('--structure-only')) process.exit(0);
if (incomplete.length) {
  console.error(`UI completion gate FAIL:\n${incomplete.map(screen => `- ${screen.id} [${screen.status}]: ${screen.openGates.join(', ')}`).join('\n')}`);
  process.exit(1);
}
