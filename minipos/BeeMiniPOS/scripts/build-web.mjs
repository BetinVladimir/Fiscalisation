import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { existsSync, readFileSync, rmSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

const variant = process.argv[2];
if (!['production', 'demo'].includes(variant) || process.argv.length !== 3) {
  console.error('Usage: node scripts/build-web.mjs production|demo');
  process.exit(1);
}
const eas = JSON.parse(readFileSync(resolve('eas.json'), 'utf8'));
const store = JSON.parse(readFileSync(resolve('store/app.json'), 'utf8'));
const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('EXPO_PUBLIC_')));
Object.assign(env, eas.build?.[variant]?.env ?? {}, store.publicEnv?.[variant] ?? {}, {
  CI: '1', NODE_ENV: 'production', EXPO_NO_DOTENV: '1', APP_VARIANT: variant,
});
const require = createRequire(resolve('package.json'));
const output = resolve('dist-cloudflare', variant);
const cacheTmp = resolve('.cache/web-build', variant);
mkdirSync(cacheTmp, { recursive: true });
env.TMPDIR = cacheTmp;
rmSync(output, { recursive: true, force: true });
const result = spawnSync(process.execPath, [require.resolve('expo/bin/cli'), 'export', '--platform', 'web', '--clear', '--output-dir', output], { env, stdio: 'inherit' });
if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status ?? 1);
if (!existsSync(resolve(output, 'index.html')) || existsSync(resolve(output, 'server'))) throw new Error('Expected static Expo export with index.html.');
