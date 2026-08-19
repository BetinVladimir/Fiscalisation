import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';

const REPO_ROOT = path.resolve(__dirname, '../../..');
const MINIPOSWEB_DIR = path.join(REPO_ROOT, 'minipos/miniposweb');
const BEEMINIPOS_DIR = path.join(REPO_ROOT, 'minipos/BeeMiniPOS');

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  fullyParallel: false,
  retries: 0,
  timeout: 45_000,
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',
  use: {
    headless: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'miniposweb',
      testMatch: 'miniposweb/**/*.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://localhost:4173',
      },
    },
    {
      name: 'beeminipos',
      testMatch: 'beeminipos/**/*.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://localhost:19100',
      },
    },
  ],
  webServer: [
    {
      command: 'npx vite --port 4173',
      url: 'http://localhost:4173',
      cwd: MINIPOSWEB_DIR,
      reuseExistingServer: true,
      timeout: 90_000,
    },
    {
      command: 'node e2e/serve.mjs',
      url: 'http://localhost:19100',
      cwd: BEEMINIPOS_DIR,
      reuseExistingServer: true,
      timeout: 30_000,
    },
  ],
});
