import { defineConfig } from '@playwright/test';
import base from './playwright.config';

export default defineConfig(base, {
  testMatch: '**/auth-wcag.spec.ts',
  testIgnore: [],
});
