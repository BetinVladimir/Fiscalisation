import { defineConfig } from '@playwright/test';
export default defineConfig({testDir:'tests/browser',testMatch:'execution-authorization.spec.mjs',workers:1,use:{baseURL:'http://127.0.0.1:4397',headless:true},webServer:{command:'node tests/browser-server.mjs',port:4397,reuseExistingServer:false}});
