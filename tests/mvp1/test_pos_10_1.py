"""POS-10.1 — Money and rounding: decimal.ts uses half-up, no float paths."""
import subprocess
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


class TestPos101(unittest.TestCase):
    def test_decimal_src_exists(self):
        src = POS_APP / 'src/shared/utils/decimal.ts'
        self.assertTrue(src.exists(), f'decimal.ts not found at {src}')

    def test_decimal_src_does_not_use_parseFloat(self):
        src = POS_APP / 'src/shared/utils/decimal.ts'
        content = src.read_text()
        self.assertNotIn('parseFloat', content,
                         'decimal.ts must not use parseFloat (invariant 1)')

    def test_decimal_src_does_not_use_Number_arithmetic(self):
        src = POS_APP / 'src/shared/utils/decimal.ts'
        content = src.read_text()
        found = re.findall(r'\bNumber\s*\(', content)
        self.assertEqual(found, [],
                         f'decimal.ts must not use Number() (invariant 1): found {found}')

    def test_golden_decimal_tests_pass(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'decimal.golden',
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
        m = re.search(r'Tests:\s+(\d+) passed', combined)
        self.assertIsNotNone(m, 'No test count found')
        self.assertGreaterEqual(int(m.group(1)), 4, 'Expected ≥4 golden decimal tests')

    def test_calculation_engine_tests_pass(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'CalculationEngine|PricingGolden',
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
        self.assertIn('PASS', combined, 'CalculationEngine or PricingGolden tests did not pass')


if __name__ == '__main__':
    unittest.main()
