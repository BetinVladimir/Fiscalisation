import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { copyFile, mkdir, readFile, readdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

const root = 'test-results';
const output = 'docs/ui-optimization-evidence/auth-wcag';
const entries = [];
for (const directory of await readdir(root)) {
  const source = join(root, directory);
  let result;
  try { result = JSON.parse(await readFile(join(source, 'axe.json'), 'utf8')); } catch { continue; }
  const state = result.state ?? 'email';
  const folder = join(output, `${state}-${result.size}-${result.mode}`);
  await mkdir(folder, { recursive: true });
  await copyFile(join(source, 'axe.json'), join(folder, 'axe.json'));
  await copyFile(join(source, 'screen.png'), join(folder, 'screen.png'));
  entries.push({
    state, size: result.size, mode: result.mode,
    violations: result.violations.reduce((count, item) => count + item.nodes.length, 0),
    incomplete: result.incomplete.reduce((count, item) => count + item.nodes.length, 0),
    evidence: folder,
  });
}
entries.sort((a, b) => `${a.state}-${a.size}-${a.mode}`.localeCompare(`${b.state}-${b.size}-${b.mode}`));
const expectedStates = ['email', 'language-open', 'otp', 'company'];
const expectedSizes = ['xs', 'md', 'lg'];
const expectedModes = ['light', 'dark'];
for (const state of expectedStates) for (const size of expectedSizes) for (const mode of expectedModes) {
  if (!entries.some(item => item.state === state && item.size === size && item.mode === mode)) throw new Error(`missing ${state}/${size}/${mode}`);
}
if (entries.length !== 24) throw new Error(`expected 24 scans, found ${entries.length}`);
const html = await readFile('.playwright-e2e-web/index.html', 'utf8');
const bundle = html.match(/AppEntry-[a-f0-9]+\.js/)?.[0];
if (!bundle) throw new Error('web bundle hash missing');
const testSha256 = createHash('sha256').update(await readFile('e2e/auth-wcag.spec.ts')).digest('hex');
const summary = {
  date: new Date().toISOString(),
  sourceCommit: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
  testSha256, bundle,
  command: 'npm run test:e2e:auth-wcag',
  commandStatus: 'FAIL: color-contrast incomplete requires manual adjudication',
  scans: entries.length,
  totalViolations: entries.reduce((count, item) => count + item.violations, 0),
  totalIncomplete: entries.reduce((count, item) => count + item.incomplete, 0),
  entries,
};
await writeFile(join(output, 'summary.json'), `${JSON.stringify(summary, null, 2)}\n`);
process.stdout.write(`${summary.scans} scans; ${summary.totalViolations} violations; ${summary.totalIncomplete} incomplete nodes\n`);
