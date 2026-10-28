import { defineConfig } from "vitest/config";
export default defineConfig({
  test: {
    include: ["src/fiscal/webBleRoute.test.ts"],
  },
});
