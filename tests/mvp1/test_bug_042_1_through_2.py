"""BUG-042.1-2 — NHIF payment type rejected before I/O when device lacks capability."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'


class TestBUG042_12(unittest.TestCase):
    def test_nhif_capability_matrix(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...',
             '-run', 'TestPaymentTypeCapabilityMatrix',
             '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0,
                         f'rc={r.returncode}\n{r.stdout}\n{r.stderr}')
        self.assertIn('PASS', r.stdout)

    def test_nhif_without_capability_rejected(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...',
             '-run', 'TestNHIFPaymentWithoutCapabilityRejectedBeforeIO',
             '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0,
                         f'rc={r.returncode}\n{r.stdout}\n{r.stderr}')
        self.assertIn('PASS', r.stdout)


if __name__ == '__main__':
    unittest.main()
