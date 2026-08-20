import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { extname, join, normalize } from 'node:path';

const dir = new URL('../.playwright-e2e-web', import.meta.url).pathname;
const port = Number(process.env.E2E_PORT || 19200);
if (!existsSync(dir)) {
  console.error(`Missing E2E build: ${dir}. Run npm run build:e2e first.`);
  process.exit(1);
}
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.png': 'image/png', '.svg': 'image/svg+xml', '.woff': 'font/woff', '.woff2': 'font/woff2' };
createServer(async (req, res) => {
  const pathname = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
  const rel = normalize(pathname).replace(/^(\.\.(\/|\\|$))+/, '').replace(/^[/\\]+/, '');
  let file = join(dir, rel || 'index.html');
  try {
    if ((await stat(file)).isDirectory()) file = join(file, 'index.html');
    res.writeHead(200, { 'Content-Type': mime[extname(file)] || 'application/octet-stream' });
    res.end(await readFile(file));
  } catch {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(await readFile(join(dir, 'index.html')));
  }
}).listen(port, '127.0.0.1', () => console.log(`BeeFiscalApp E2E server on http://127.0.0.1:${port}`));
