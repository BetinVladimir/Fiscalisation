"""TST-POS-06.1 — gateway finalization: CARD+CASH intent → two payments with types; missing → 422."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_GATEWAY = ROOT / '../POS/pos-gateway'


class TestTstPos061(unittest.TestCase):
    def test_finalize_command_carries_all_payments(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestFinalizeCommand',
             '-count=1', './internal/drain/...'],
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
        self.assertIn('PASS', combined,
                      'TestFinalizeCommand* tests did not pass')
        self.assertIn('TestFinalizeCommandCarriesAllPaymentsAndTypes', combined,
                      'TestFinalizeCommandCarriesAllPaymentsAndTypes not found in output')

    def test_finalize_rejects_empty_payments(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestFinalizeCommandRejectsEmpty',
             '-count=1', './internal/drain/...'],
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
        self.assertIn('PASS', combined, 'Rejection tests did not pass')


if __name__ == '__main__':
    unittest.main()
