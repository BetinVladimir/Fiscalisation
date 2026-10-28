"""TST-POS-07.1 — gateway Submit: VAT not recalculated; idempotency key; DB error → 503."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_GATEWAY = ROOT / '../POS/pos-gateway'


class TestTstPos071(unittest.TestCase):
    def test_submit_tests_pass(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestSubmit_',
             '-count=1', './internal/handler/...'],
            cwd=str(POS_GATEWAY),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-3000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'TestSubmit_* tests did not pass')

    def test_vat_not_recalculated_as_20_percent(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestSubmit_doesNotAssume20PercentVAT',
             '-count=1', './internal/handler/...'],
            cwd=str(POS_GATEWAY),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-2000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'VAT recalculation guard test did not pass')

    def test_db_error_on_policy_check_returns_503(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestSubmit_failsClosedOnPolicyLookupError',
             '-count=1', './internal/handler/...'],
            cwd=str(POS_GATEWAY),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-2000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'Policy lookup error test did not pass')


if __name__ == '__main__':
    unittest.main()
