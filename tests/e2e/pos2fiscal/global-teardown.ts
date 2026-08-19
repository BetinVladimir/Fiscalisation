import { readFileSync, unlinkSync } from 'node:fs';
import path from 'node:path';

const PID_FILE = path.join(__dirname, '.device-mock-pids.json');

export default async function globalTeardown() {
  try {
    const pids: { s3?: number; bc?: number } = JSON.parse(readFileSync(PID_FILE, 'utf-8'));
    for (const pid of [pids.s3, pids.bc]) {
      if (pid) {
        try { process.kill(pid, 'SIGTERM'); } catch { /* already gone */ }
      }
    }
    unlinkSync(PID_FILE);
  } catch {
    // pid file missing or processes already exited
  }
}
