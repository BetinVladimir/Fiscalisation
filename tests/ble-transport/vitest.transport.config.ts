// Proposed transport-only config. No application Vite config or React/UI plugin.
import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL(".", import.meta.url));
export default defineConfig({
  root,
  plugins: [],
  cacheDir: ".cache/vite-transport",
  server: { fs: { strict: true, allow: [root] } },
  test: {
    environment: "node",
    include: ["minipos/miniposweb/tests/webBleLifecycle.test.ts"],
    passWithNoTests: false,
    pool: "forks",
    fileParallelism: false,
    minWorkers: 1,
    maxWorkers: 1,
    testTimeout: 5000,
    hookTimeout: 10000,
  },
});
