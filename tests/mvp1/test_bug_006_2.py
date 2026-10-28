"""BUG-006.2 — receipt_saga.go deleted; tests green."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'


class TestBUG006_2(unittest.TestCase):
    def test_saga_file_absent(self):
        f = BACKEND / 'internal/domain/receipt_saga.go'
        self.assertFalse(f.exists(),
                         f'receipt_saga.go should be deleted but still exists: {f}')

    def test_build_passes(self):
        r = subprocess.run(['go', 'build', './...'],
                           cwd=str(BACKEND), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0,
                         f'go build failed:\n{r.stdout}\n{r.stderr}')

    def test_domain_tests_pass(self):
        r = subprocess.run(['go', 'test', './internal/domain/...', '-count=1', '-timeout=60s'],
                           cwd=str(BACKEND), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0,
                         f'go test rc={r.returncode}\n{r.stdout}\n{r.stderr}')


if __name__ == '__main__':
    unittest.main()
