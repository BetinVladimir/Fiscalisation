import { chromium } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const baseURL = process.env.WCAG_BASE_URL || "https://demo-fiscal.beeloy.org";
const roots = ['app', 'src/app'];
const viewports = [
  { name: 'xs', width: 360, height: 800 },
  { name: 'md', width: 960, height: 900 },
  { name: 'lg', width: 1440, height: 1000 },
];

function collectRoutes() {
  const routes = new Set(['/']);
  const walk = (dir, root) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(file, root);
      else if (/\.(tsx|jsx|ts|js)$/.test(entry.name) && !entry.name.startsWith('_') && !file.includes('__tests__')) {
        let route = '/' + path.relative(root, file).replace(/\\/g, '/').replace(/\.(tsx|jsx|ts|js)$/, '');
        route = route.replace(/\/\([^/]+\)/g, '').replace(/\/index$/, '') || '/';
        if (!route.includes('[') && !route.endsWith('/App') && !route.includes('/ui/') && !route.includes('/styles')) routes.add(route);
      }
    }
  };
  for (const candidate of roots) {
    const root = path.resolve(candidate);
    if (fs.existsSync(root)) walk(root, root);
  }
  return [...routes].sort();
}

const browser = await chromium.launch({ headless: true });
const failures = [];
let scans = 0;
try {
  for (const viewport of viewports) {
    for (const colorScheme of ['light', 'dark']) {
      const context = await browser.newContext({ viewport, colorScheme, reducedMotion: 'reduce' });
      const page = await context.newPage();
      for (const route of collectRoutes()) {
        const response = await page.goto(new URL(route, baseURL).href, { waitUntil: 'networkidle', timeout: 30000 }).catch(() => null);
        if (!response || response.status() >= 400) {
          failures.push(`${viewport.name}/${colorScheme} ${route}: HTTP ${response?.status() ?? 'navigation failed'}`);
          continue;
        }
        await page.waitForTimeout(250);
        const result = await new AxeBuilder({ page }).withRules(['color-contrast']).analyze();
        scans += 1;
        for (const violation of result.violations) {
          for (const node of violation.nodes) {
            failures.push(`${viewport.name}/${colorScheme} ${route}: ${node.failureSummary}; target=${node.target.join(' ')}`);
          }
        }
      }
      await context.close();
    }
  }
} finally {
  await browser.close();
}
if (failures.length) {
  console.error(`WCAG AA contrast audit failed (${failures.length} findings across ${scans} page scans):\n${failures.join('\n')}`);
  process.exit(1);
}
console.log(`WCAG AA contrast audit passed: ${scans} page scans (${collectRoutes().length} routes × light/dark × xs/md/lg).`);
