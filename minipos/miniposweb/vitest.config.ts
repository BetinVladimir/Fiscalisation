import { defineConfig } from "vitest/config";

// Playwright specifications intentionally use the same *.spec.ts suffix, so
// keep the unit runner scoped to its own directory. Without this boundary
// Vitest imports Playwright's global suite during `npm test`.
export default defineConfig({
  test: {
    include: ["tests/**/*.test.ts"],
  },
});
