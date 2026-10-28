"""TST-POS-09.1 — decimal.golden.test.ts: 34/34 cases; Number() arithmetic banned."""
import subprocess
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'
GOLDEN_TEST = POS_APP / 'src/shared/utils/decimal.golden.test.ts'
DECIMAL_SRC = POS_APP / 'src/shared/utils/decimal.ts'


class TestTstPos091(unittest.TestCase):
    def test_golden_test_file_exists(self):
        self.assertTrue(GOLDEN_TEST.exists(),
                        f'decimal.golden.test.ts not found at {GOLDEN_TEST}')

    def test_decimal_src_does_not_use_Number_arithmetic(self):
        """decimal.ts must not use Number() for arithmetic on money values."""
        content = DECIMAL_SRC.read_text()
        # Number() is banned for arithmetic per invariant 1
        # Allow Number() for non-arithmetic use (e.g., Number.isFinite)
        forbidden = re.findall(r'\bNumber\s*\(', content)
        self.assertEqual(
            forbidden, [],
            f'decimal.ts must not use Number() for arithmetic: found {forbidden}',
        )

    def test_golden_tests_pass(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'shared/utils/decimal.golden',
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
        self.assertRegex(combined, r'Tests:\s+\d+ passed',
                         'No test count found in jest output')

    def test_rounding_golden_tests_pass(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'roundingGolden',
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
        self.assertIn('PASS', combined, 'No PASS in rounding golden tests')


if __name__ == '__main__':
    unittest.main()
