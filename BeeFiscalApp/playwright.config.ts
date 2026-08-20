import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
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
