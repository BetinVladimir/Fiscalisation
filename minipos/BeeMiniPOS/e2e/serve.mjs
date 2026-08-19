/**
 * Static file server for Playwright E2E tests.
 *
 * Build the web bundle first:
 *   npx expo export --platform web --output-dir .playwright-e2e-web
 *
 * Then run:
 *   node e2e/serve.mjs
 */
import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { extname, join, normalize } from 'node:path';

const dir = new URL('../.playwright-e2e-web', import.meta.url).pathname;
const port = Number(process.env.E2E_PORT || 19100);

if (!existsSync(dir)) {
  console.error(`[e2e/serve] Missing build directory: ${dir}`);
  console.error('[e2e/serve] Run: npx expo export --platform web --output-dir .playwright-e2e-web');
  process.exit(1);
}

const mime = {
  '.html': 'text/html',
  '.js':   'text/javascript',
  '.css':  'text/css',
  '.json': 'application/json',
  '.png':  'image/png',
  '.svg':  'image/svg+xml',
  '.ico':  'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
};

const server = createServer(async (req, res) => {
  const p = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
  const rel = normalize(p).replace(/^(\.\.(\/|\\|$))+/, '').replace(/^[/\\]+/, '');
  let file = join(dir, rel || 'index.html');
  try {
    if ((await stat(file)).isDirectory()) file = join(file, 'index.html');
    const data = await readFile(file);
    res.writeHead(200, { 'Content-Type': mime[extname(file)] || 'application/octet-stream' });
    res.end(data);
  } catch {
    // SPA fallback — return index.html for unknown routes
    try {
      res.writeHead(200, { 'Content-Type': 'text/html' });
      res.end(await readFile(join(dir, 'index.html')));
    } catch {
      res.writeHead(404).end('not found');
    }
  }
});

server.listen(port, '127.0.0.1', () => {
  console.log(`BeeMiniPOS E2E server on http://127.0.0.1:${port}`);
});
