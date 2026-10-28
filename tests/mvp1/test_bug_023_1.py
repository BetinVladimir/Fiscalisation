"""BUG-023.1 — rounding_golden.json has ≥30 vectors with required fields and boundary cases."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
GOLDEN = ROOT / 'docs/MVP1/fixtures/rounding_golden.json'


class TestBUG0231(unittest.TestCase):
    def setUp(self):
        self.assertTrue(GOLDEN.exists(), f'rounding_golden.json not found: {GOLDEN}')
        with open(GOLDEN) as f:
            self.vectors = json.load(f)

    def test_minimum_30_vectors(self):
        self.assertGreaterEqual(
            len(self.vectors), 30,
            f'golden must have ≥30 vectors, got {len(self.vectors)}'
        )

    def test_all_vectors_have_required_fields(self):
        required = {'name', 'unit_cents', 'qty_milli', 'discount_cents', 'expect_line_cents'}
        for i, v in enumerate(self.vectors):
            missing = required - set(v.keys())
            self.assertFalse(missing, f'vector[{i}] missing fields: {missing}')

    def test_all_values_are_integers(self):
        for v in self.vectors:
            for field in ('unit_cents', 'qty_milli', 'discount_cents', 'expect_line_cents'):
                val = v[field]
                self.assertIsInstance(
                    val, int,
                    f'vector {v["name"]}: {field} must be int, got {type(val).__name__}'
                )

    def test_half_up_boundary_case_q500(self):
        # u1_q500: (1*500+500)/1000 = 1 → 1 cent (half-up rounds 0.005 up)
        matching = [v for v in self.vectors if v['unit_cents'] == 1 and v['qty_milli'] == 500]
        self.assertTrue(matching, 'golden must contain u1_q500_d0 boundary vector')
        for v in matching:
            if v['discount_cents'] == 0:
                self.assertEqual(
                    v['expect_line_cents'], 1,
                    '0.01 × 0.500 must round to 1 cent (half-up)'
                )

    def test_formula_consistency(self):
        for v in self.vectors:
            gross = (v['unit_cents'] * v['qty_milli'] + 500) // 1000
            net = gross - v['discount_cents']
            self.assertEqual(
                net, v['expect_line_cents'],
                f'vector {v["name"]}: formula ({v["unit_cents"]}×{v["qty_milli"]}+500)÷1000 - {v["discount_cents"]} = {net} ≠ {v["expect_line_cents"]}'
            )


if __name__ == '__main__':
    unittest.main()
