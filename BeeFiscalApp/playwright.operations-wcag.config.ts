import { defineConfig } from '@playwright/test';
import base from './playwright.config';

export default defineConfig(base, {
  testMatch: '**/operations-wcag.spec.ts',
  testIgnore: [],
});
