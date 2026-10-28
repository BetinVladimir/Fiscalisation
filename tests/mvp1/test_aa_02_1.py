"""AA-02.1 — idempotentRequest module + key stability tests in AdminApp."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ADMIN = ROOT / 'AdminApp'


class TestAA02_1(unittest.TestCase):
    def test_jest_idempotent_request(self):
        r = subprocess.run(
            ['npx', 'jest', 'src/idempotentRequest.test.ts', '--no-coverage'],
            cwd=str(ADMIN), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('5 passed', r.stdout + r.stderr)


if __name__ == '__main__':
    unittest.main()
