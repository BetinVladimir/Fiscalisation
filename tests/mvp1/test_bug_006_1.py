"""BUG-006.1 — receipt_plan.go exists with plan functions; build+tests green."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'


class TestBUG006_1(unittest.TestCase):
    def test_receipt_plan_file_exists(self):
        f = BACKEND / 'internal/domain/receipt_plan.go'
        self.assertTrue(f.exists(), f'receipt_plan.go not found: {f}')
        content = f.read_text()
        for sym in ('ReceiptFinalizePlan', 'ValidateReceiptPlan',
                    'supportedPaymentType', 'receiptMoneyCents', 'ReceiptPlanHash'):
            self.assertIn(sym, content, f'Missing symbol {sym!r} in receipt_plan.go')

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
