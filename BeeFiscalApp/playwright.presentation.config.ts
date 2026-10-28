import {defineConfig,devices} from '@playwright/test';
import {validatePresentationBuild} from './e2e/presentation-build-guard';

// This suite requires a separately built device-data-presentation-host.tsx.
// Verify the synthetic export manifest; never reuse an existing server.
const directory=validatePresentationBuild(process.env.BEEFISCAL_PRESENTATION_DIR);
export default defineConfig({
 testDir:'./e2e',testMatch:'**/device-data-readable.spec.ts',fullyParallel:false,retries:0,
 reporter:[['list'],['json',{outputFile:'presentation-results.json'}]],
 use:{headless:true,baseURL:'http://127.0.0.1:19209',testIdAttribute:'data-testid'},
 projects:[{name:'chromium',use:{...devices['Desktop Chrome']}}],
 webServer:{command:'node e2e/serve.mjs',port:19209,reuseExistingServer:false,
  env:{E2E_PORT:'19209',E2E_WEB_DIR:directory}},
});
