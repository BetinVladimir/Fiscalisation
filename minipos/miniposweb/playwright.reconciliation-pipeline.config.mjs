import {defineConfig} from '@playwright/test';
export default defineConfig({testDir:'tests/browser',testMatch:'reconciliation-http-pipeline.spec.mjs',workers:1,timeout:120000,use:{baseURL:'http://127.0.0.1:4398',headless:true},webServer:{command:'./node_modules/.bin/vite --host 127.0.0.1 --port 4398',port:4398,reuseExistingServer:false}});
