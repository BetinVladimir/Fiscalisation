#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const root = path.resolve(__dirname, '..');
process.chdir(root);
const read = p => JSON.parse(fs.readFileSync(p, 'utf8'));
const save = (p, value) => fs.writeFileSync(p, JSON.stringify(value, null, 2) + '\n');
const manifestFile = path.join(__dirname, 'app.json');
const manifest = read(manifestFile);
const eas = read('eas.json');
const [action = 'check', platform = 'all', variant = 'production', ...options] = process.argv.slice(2);
const option = key => { const i = process.argv.indexOf(key); return i < 0 ? undefined : process.argv[i + 1]; };
const dryRun = process.argv.includes('--dry-run');
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
function environment(v) {
  const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.startsWith('EXPO_PUBLIC_')));
  return { ...env, ...eas.build[v].env, ...manifest.publicEnv?.[v], APP_VARIANT: v, EXPO_NO_DOTENV: '1' };
}
function command(args, env, capture = false) {
  console.log('$ eas ' + args.join(' '));
  if (dryRun) return { status: 0, stdout: '', stderr: '' };
  // A pinned CLI keeps developer machines and CI on the same commands/schema.
  const result = cp.spawnSync('npx', ['--yes', 'eas-cli@20.3.0', ...args], { cwd: root, env, encoding: 'utf8', stdio: capture ? 'pipe' : 'inherit' });
  if (result.error) throw result.error;
  return result;
}
function config(v) {
  const r = cp.spawnSync(process.execPath, [require.resolve('expo/bin/cli'), 'config', '--type', 'public', '--json'], { cwd: root, env: environment(v), encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 });
  if (r.status !== 0) throw new Error(r.stderr || 'Expo config failed');
  return JSON.parse(r.stdout);
}
function check() {
  const errors = [], external = [];
  const pkg = read('package.json');
  if (!/54\./.test(pkg.dependencies.expo)) errors.push('Expected Expo SDK 54 for the validated release toolchain');
  for (const v of ['production', 'demo']) {
    const c = config(v), id = `org.beeloy.${manifest.app}.${v === 'demo' ? 'demo' : 'prod'}`;
    if (c.android?.package !== id || c.ios?.bundleIdentifier !== id) errors.push(`${v}: unexpected Android/iOS identifier`);
    if (eas.build[v].distribution !== 'store' || eas.build[v].android?.buildType !== 'app-bundle') errors.push(`${v}: requires store/AAB profile`);
    if (!eas.build[v].autoIncrement || eas.cli.appVersionSource !== 'remote') errors.push(`${v}: remote build numbering missing`);
    if (!c.icon || !fs.existsSync(path.resolve(root, c.icon))) errors.push(`${v}: app icon missing`);
    if (!c.extra?.eas?.projectId) external.push(`${v}: run npm run store:setup to link an EAS project`);
    if (!manifest.apple[v].ascAppId) external.push(`${v}: create App Store Connect record for ${id}, then set its numeric Apple ID`);
    console.log(`${v}: ${id} | ${c.extra.webOrigin}`);
  }
  if (manifest.app === 'fiscaladmin') {
    for (const v of ['production','demo']) if (!manifest.publicEnv?.[v]?.EXPO_PUBLIC_PLATFORM_OIDC_ISSUER || !manifest.publicEnv?.[v]?.EXPO_PUBLIC_PLATFORM_OIDC_CLIENT_ID) external.push(`${v}: configure fiscaladmin OIDC issuer/client ID in store/app.json publicEnv`);
  }
  console.log(JSON.stringify({ localErrors: errors, externalSetup: external }, null, 2));
  if (errors.length) throw new Error('Local release checks failed');
}
function syncSubmission() {
  for (const v of ['production', 'demo']) {
    eas.build[v].env = { ...eas.build[v].env, ...manifest.publicEnv?.[v] };
    const asc = manifest.apple[v].ascAppId;
    if (asc) {
      if (!/^\d+$/.test(asc)) throw new Error('ascAppId must be the numeric App Store Connect Apple ID');
      eas.submit[v].ios.ascAppId = asc;
    }
  }
  save('eas.json', eas);
}
async function main() {
  if (action === 'check') return check();
  if (action === 'setup') {
    const id = option('--project-id');
    if (id) { if (!uuid.test(id)) throw new Error('Invalid EAS project UUID'); manifest.projectId = id; }
    for (const [v, flag] of [['production','--asc-prod'],['demo','--asc-demo']]) {
      const asc = option(flag); if (asc) manifest.apple[v].ascAppId = asc;
    }
    if (!dryRun) { save(manifestFile, manifest); syncSubmission(); }
    if (!manifest.projectId) {
      const r = command(['init','--non-interactive','--force'], environment('production'), true);
      const output = (r.stdout || '') + (r.stderr || '');
      console.log(output);
      // EAS prints this exact JSON property when it cannot edit dynamic app.config.
      const match = output.match(/"projectId"\s*:\s*"([0-9a-f-]{36})"/i);
      if (match && uuid.test(match[1])) { manifest.projectId = match[1]; save(manifestFile, manifest); }
      else if (r.status !== 0) throw new Error('EAS setup needs interactive login or a project ID; see STORE-RELEASE.md');
      if (!dryRun && !manifest.projectId) { manifest.projectId = config('production').extra?.eas?.projectId || null; save(manifestFile, manifest); }
    }
    const r = command(['project:info'], environment('production'));
    if (r.status !== 0) throw new Error('EAS project verification failed');
    return;
  }
  if (!['build','submit','deploy'].includes(action) || !['android','ios','all'].includes(platform) || !['production','demo'].includes(variant)) throw new Error('Usage: release.cjs check|setup OR build|submit|deploy android|ios|all production|demo [--dry-run] [--id BUILD_UUID]');
  if (action === 'submit' && platform === 'all') throw new Error('Submit one platform per build UUID');
  const env = environment(variant);
  const c = config(variant);
  if (!dryRun && !c.extra?.eas?.projectId) throw new Error('Run npm run store:setup first');
  if (manifest.app === 'fiscaladmin' && !dryRun && (!env.EXPO_PUBLIC_PLATFORM_OIDC_ISSUER || !env.EXPO_PUBLIC_PLATFORM_OIDC_CLIENT_ID)) throw new Error('Fiscal Admin requires real OIDC issuer and client ID in store/app.json publicEnv');
  if ((action === 'submit' || action === 'deploy') && (platform === 'ios' || platform === 'all') && !dryRun && !manifest.apple[variant].ascAppId) throw new Error(`Set Apple ID: npm run store:setup -- --asc-${variant === 'demo' ? 'demo' : 'prod'} NUMERIC_ID`);
  if (!dryRun) {
    syncSubmission();
    if (platform === 'android' || platform === 'all') {
      const keys = read(path.join(__dirname, 'credentials.local.json'));
      if (!keys[variant]?.android) throw new Error('Run npm run store:keys first');
      fs.writeFileSync('credentials.json', JSON.stringify(keys[variant], null, 2) + '\n', { mode: 0o600 });
    }
  }
  const args = [action === 'submit' ? 'submit' : 'build', '--platform', platform, '--profile', variant];
  if (action === 'submit') {
    const id = option('--id');
    if (!id || !uuid.test(id)) throw new Error('submit requires --id BUILD_UUID; never chooses an ambiguous latest build');
    if (!dryRun) {
      const r = command(['build:view',id,'--json'],env,true);
      if (r.status !== 0) throw new Error('Cannot inspect selected build');
      const b = JSON.parse(r.stdout);
      if (b.buildProfile !== variant || b.project?.id !== c.extra.eas.projectId || b.platform?.toLowerCase() !== platform || b.status !== 'FINISHED') throw new Error('Selected build does not match project, variant, platform or FINISHED status');
    }
    args.push('--id',id);
  }
  if (action === 'deploy') args.push('--auto-submit-with-profile',variant);
  if (process.argv.includes('--non-interactive')) args.push('--non-interactive');
  if (process.argv.includes('--no-wait')) args.push('--no-wait');
  const result = command(args,env);
  if (result.status !== 0) process.exit(result.status ?? 1);
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });
