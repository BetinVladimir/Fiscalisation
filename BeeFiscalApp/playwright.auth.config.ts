import { defineConfig } from '@playwright/test';
import base from './playwright.config';

export default defineConfig(base, {
  testMatch: ['**/auth-ownership.spec.ts', '**/auth-paper.spec.ts', '**/mvp-auth-shell-en.spec.ts'],
  testIgnore: [],
});
