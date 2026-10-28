"""AA-01.1 — AdminApp: expo-crypto idempotency.ts newKey() — 4 Jest tests pass."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
ADMINAPP = ROOT / 'AdminApp'


class TestAA01_1(unittest.TestCase):
    def test_idempotency_jest_suite(self):
        r = subprocess.run(
            ['npx', '--no-install', 'jest', '--runInBand', 'src/idempotency.test.ts'],
            cwd=str(ADMINAPP),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            r.returncode, 0,
            f'jest exited {r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1000:]}',
        )
        combined = r.stdout + r.stderr
        self.assertIn(
            'Tests:       4 passed', combined,
            f'Expected 4 passing tests:\nstdout={r.stdout}\nstderr={r.stderr}',
        )


if __name__ == '__main__':
    unittest.main()
