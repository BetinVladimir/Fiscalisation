import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  testIgnore: ['**/device-data-readable.spec.ts', '**/auth-ownership.spec.ts', '**/mvp-auth-shell-en.spec.ts', '**/contrast-stack-diagnostic.spec.ts', '**/reduced-motion.spec.ts', '**/auth-paper.spec.ts', '**/auth-wcag.spec.ts', '**/operations-wcag.spec.ts', '**/reports-wcag.spec.ts', '**/audit-wcag.spec.ts', '**/settings-wcag.spec.ts', '**/devices-admin-wcag.spec.ts'],
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    headless: true,
    baseURL: 'http://127.0.0.1:19200',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    testIdAttribute: 'data-testid',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 1000 } } }],
  webServer: {
    command: 'node e2e/serve.mjs',
    port: 19200,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
});
