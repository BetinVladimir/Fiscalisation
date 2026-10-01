import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
export default defineConfig({
  plugins: [react()],
  define: { global: "globalThis" },
  resolve: {
    alias: [{ find: /^react-native$/, replacement: "react-native-web" }],
    extensions: [".web.mjs", ".mjs", ".web.js", ".js", ".web.tsx", ".tsx", ".web.ts", ".ts", ".json"],
  },
  optimizeDeps: { include: ["@beeloy/design-tokens"], esbuildOptions: { resolveExtensions: [".web.mjs", ".mjs", ".web.js", ".js", ".web.tsx", ".tsx", ".web.ts", ".ts", ".json"] } },
  base: "./",
  build: { assetsDir: "assets", sourcemap: false, commonjsOptions: { include: [/node_modules/, /BeeloyBackend\/mobile\/design-tokens/] } },
  server: { host: "0.0.0.0", port: 4173, fs: { allow: ["..", "../../../BeeloyBackend/mobile/design-tokens"] } },
});
