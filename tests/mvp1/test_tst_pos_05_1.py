"""TST-POS-05.1 — write timeout → FiscalUnknownError; one getOperationStatus, no re-submit."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


class TestTstPos051(unittest.TestCase):
    def test_write_timeout_unknown(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'src/__tests__/tst-pos-05',
             '--verbose'],
            cwd=str(POS_APP),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'jest rc=' + str(result.returncode) + '\n' + result.stdout[-3000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'No PASS in jest output')
        self.assertIn('FiscalUnknownError', combined,
                      'FiscalUnknownError not mentioned in test output')
        self.assertRegex(combined, r'Tests:\s+\d+ passed',
                         'No test count found')


if __name__ == '__main__':
    unittest.main()
