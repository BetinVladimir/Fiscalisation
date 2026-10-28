"""POS-13.1 — Refund uses originalFiscalReference from original sale."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


class TestPos131(unittest.TestCase):
    def test_refund_uses_original_fiscal_reference(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'refund.originalReference',
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
        self.assertIn('originalFiscalReference', combined,
                      'originalFiscalReference not mentioned in test output')
        import re
        m = re.search(r'Tests:\s+(\d+) passed', combined)
        self.assertIsNotNone(m, 'No test count found')
        self.assertGreaterEqual(int(m.group(1)), 2,
                                'Expected ≥2 refund reference tests')


if __name__ == '__main__':
    unittest.main()
