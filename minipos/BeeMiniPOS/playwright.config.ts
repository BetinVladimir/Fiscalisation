import { defineConfig, devices } from '@playwright/test';

/**
 * Build the web bundle before running:
 *   npx expo export --platform web --output-dir .playwright-e2e-web
 *
 * Then run tests:
 *   npx playwright test
 */
export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    headless: true,
    baseURL: 'http://127.0.0.1:19100',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    trace: 'retain-on-failure',
    // React Native Web emits testID as data-testid
    testIdAttribute: 'data-testid',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // BLE tests need a real browser context; other tests are fine headless
        viewport: { width: 1440, height: 900 },
      },
    },
  ],
  webServer: {
    command: 'node e2e/serve.mjs',
    port: 19100,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
});
