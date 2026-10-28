"""AA-02.2 — AdminApp/App.tsx + idempotentRequest.ts pass TypeScript check."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ADMIN = ROOT / 'AdminApp'


class TestAA02_2(unittest.TestCase):
    def test_tsc_no_emit(self):
        r = subprocess.run(
            ['npx', 'tsc', '--noEmit'],
            cwd=str(ADMIN), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'tsc failed:\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')


if __name__ == '__main__':
    unittest.main()
