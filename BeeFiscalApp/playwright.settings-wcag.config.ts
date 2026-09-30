import { defineConfig } from '@playwright/test';
import base from './playwright.config';

export default defineConfig(base, {
  testMatch: '**/settings-wcag.spec.ts',
  testIgnore: [],
});
