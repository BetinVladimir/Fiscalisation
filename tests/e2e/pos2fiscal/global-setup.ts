import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import path from 'node:path';

const MOCK_PATH = path.resolve(__dirname, '../full-fiscal/device-mock.mjs');
const PID_FILE = path.join(__dirname, '.device-mock-pids.json');

async function waitReady(url: string, maxMs = 12_000): Promise<void> {
  const deadline = Date.now() + maxMs;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error(`Device mock at ${url} did not become ready within ${maxMs}ms`);
}

export default async function globalSetup() {
  const s3 = spawn('node', [MOCK_PATH], {
    env: { ...process.env, DEVICE_PROFILE: 'edge-agent-s3', HTTP_PORT: '19001' },
    stdio: 'pipe',
    detached: false,
  });
  const bc = spawn('node', [MOCK_PATH], {
    env: { ...process.env, DEVICE_PROFILE: 'bluecash-app', HTTP_PORT: '19002' },
    stdio: 'pipe',
    detached: false,
  });

  s3.stderr?.on('data', (d) => process.stderr.write(`[mock:s3] ${d}`));
  bc.stderr?.on('data', (d) => process.stderr.write(`[mock:bc] ${d}`));

  writeFileSync(PID_FILE, JSON.stringify({ s3: s3.pid, bc: bc.pid }));

  await Promise.all([
    waitReady('http://localhost:19001/__e2e/operations'),
    waitReady('http://localhost:19002/__e2e/operations'),
  ]);

  console.log('[pos2fiscal] device mocks ready: edge-agent-s3 on :19001, bluecash-app on :19002');
}
