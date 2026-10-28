"""DSY-05.1 — storno_matrix.json has 7 test cases with correct structure."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY05_1(unittest.TestCase):
    def test_storno_matrix_has_7_cases(self):
        f = ROOT / 'docs/MVP1/fixtures/storno_matrix.json'
        self.assertTrue(f.exists(), f'File not found: {f}')
        data = json.loads(f.read_text())
        self.assertIsInstance(data, list, 'storno_matrix.json must be a JSON array')
        self.assertEqual(len(data), 7, f'Expected 7 cases, got {len(data)}: {data}')

    def test_storno_matrix_case_structure(self):
        f = ROOT / 'docs/MVP1/fixtures/storno_matrix.json'
        data = json.loads(f.read_text())
        for i, case in enumerate(data):
            self.assertIn('name', case, f'Case {i} missing "name"')
            # Each case either expects "OK" outcomes or error outcomes
            self.assertIn('expect', case, f'Case {i} missing "expect"')
            expects = case['expect']
            self.assertIsInstance(expects, list, f'Case {i} expect must be a list')
            self.assertGreater(len(expects), 0, f'Case {i} expect must not be empty')

    def test_storno_matrix_has_error_cases(self):
        f = ROOT / 'docs/MVP1/fixtures/storno_matrix.json'
        data = json.loads(f.read_text())
        # Must have at least one case that does NOT produce "OK" (error/boundary condition)
        error_cases = [c for c in data if any(e != 'OK' for e in c.get('expect', []))]
        self.assertGreater(
            len(error_cases), 0,
            f'storno_matrix.json must have at least one non-OK case: {[c["name"] for c in data]}',
        )

    def test_storno_matrix_has_full_reversal(self):
        f = ROOT / 'docs/MVP1/fixtures/storno_matrix.json'
        data = json.loads(f.read_text())
        full_cases = [c for c in data if c.get('name') == 'full']
        self.assertEqual(len(full_cases), 1, 'Must have a "full" reversal case')
        self.assertEqual(full_cases[0]['expect'], ['OK'])


if __name__ == '__main__':
    unittest.main()
