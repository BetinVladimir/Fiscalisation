"""TG-010.1-2 — Reconciler resolves EXECUTING ops and escalates after MaxAttempts."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'

class TestTG010(unittest.TestCase):
    def test_reconciler_tests_pass(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain', '-run', 'Reconciler', '-race', '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('TestReconcilerResolvesExecutingOperations', r.stdout)
        self.assertIn('TestReconcilerEscalatesAfterMaxAttempts', r.stdout)
        self.assertIn('PASS', r.stdout)

if __name__ == '__main__':
    unittest.main()
