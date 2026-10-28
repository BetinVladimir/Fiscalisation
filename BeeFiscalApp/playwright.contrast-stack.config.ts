import { defineConfig } from '@playwright/test';
import base from './playwright.config';
export default defineConfig(base, { testMatch: '**/contrast-stack-diagnostic.spec.ts', testIgnore: [] });
