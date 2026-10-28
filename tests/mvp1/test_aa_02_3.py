"""AA-02.3 — BeeFiscalApp idempotent-request tests pass."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BEEAPP = ROOT / 'BeeFiscalApp'


class TestAA02_3(unittest.TestCase):
    def test_jest_idempotent_request_bee(self):
        r = subprocess.run(
            ['npx', 'jest', '__tests__/idempotent-request.test.ts',
             '--config', 'jest.ui.config.cjs', '--no-coverage'],
            cwd=str(BEEAPP), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('6 passed', r.stdout + r.stderr)


if __name__ == '__main__':
    unittest.main()
