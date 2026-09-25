import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { existsSync, rmSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const variant = process.argv[2];
if (!['production', 'demo'].includes(variant) || process.argv.length !== 3) {
  console.error('Usage: node ../scripts/build-web.mjs production|demo');
  process.exit(1);
}
const manifest = JSON.parse(readFileSync(resolve('store/app.json'), 'utf8'));
const publicEnv = manifest.publicEnv?.[variant] ?? {};
const api = variant === 'demo' ? 'https://demo-api.beeloy.org' : 'https://api.beeloy.org';
const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('EXPO_PUBLIC_')));
Object.assign(env, {
  CI: '1', NODE_ENV: 'production', EXPO_NO_DOTENV: '1', APP_VARIANT: variant,
  EXPO_PUBLIC_PLATFORM_API_URL: api,
  ...publicEnv,
});
const require = createRequire(resolve('package.json'));
const output = resolve('dist-cloudflare', variant);
rmSync(output, { recursive: true, force: true });
console.log(`Exporting ${variant}: ${api}`);
const result = spawnSync(process.execPath, [require.resolve('expo/bin/cli'), 'export', '--platform', 'web', '--clear', '--output-dir', output], { env, stdio: 'inherit' });
if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status ?? 1);
if (!existsSync(resolve(output, 'index.html')) || existsSync(resolve(output, 'server'))) {
  throw new Error('Expected static Expo export with index.html; server output is unsupported.');
}
