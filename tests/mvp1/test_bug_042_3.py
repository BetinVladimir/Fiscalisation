"""BUG-042.3 — C++ intent_processor only accepts CASH/CARD, rejects NHIF at parse time."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestBUG042_3(unittest.TestCase):
    def test_cpp_restricts_to_cash_card(self):
        """Verify intent_processor.cpp source restricts payment types to CASH/CARD."""
        f = ROOT / 'IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp'
        self.assertTrue(f.exists(), f'intent_processor.cpp not found: {f}')
        content = f.read_text()
        # The guard line must be present
        self.assertIn('v.type != "CASH" && v.type != "CARD"', content,
                      'C++ must reject non-CASH/CARD payment types at decode stage')

    def test_go_backend_validates_capability_before_io(self):
        """Verify Go backend has ValidatePaymentTypeForDevice function."""
        cap_file = ROOT / 'fiscal-backend/internal/domain/payment_capability.go'
        self.assertTrue(cap_file.exists(), f'payment_capability.go not found')
        content = cap_file.read_text()
        self.assertIn('ValidatePaymentTypeForDevice', content)
        self.assertIn('PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE', content)

    def test_full_domain_suite(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...', '-count=1', '-timeout=60s'],
            cwd=str(ROOT / 'fiscal-backend'),
            capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0,
                         f'rc={r.returncode}\n{r.stdout}\n{r.stderr}')


if __name__ == '__main__':
    unittest.main()
