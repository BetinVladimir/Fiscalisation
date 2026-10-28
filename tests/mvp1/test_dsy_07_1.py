"""DSY-07.1 — payment_slots_daisy.json has >=3 slot definitions."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY07_1(unittest.TestCase):
    def test_payment_slots_fixture(self):
        f = ROOT / 'docs/MVP1/fixtures/payment_slots_daisy.json'
        self.assertTrue(f.exists(), f'File not found: {f}')
        data = json.loads(f.read_text())
        slots = data.get('slots', [])
        self.assertGreaterEqual(
            len(slots), 3,
            f'Expected >=3 slots, got {len(slots)}: {slots}',
        )
        types = [s.get('type') for s in slots]
        self.assertIn('CASH', types, f'CASH slot missing: {types}')
        self.assertIn('CARD', types, f'CARD slot missing: {types}')
        # Every slot has index, type, name, code
        for s in slots:
            self.assertIn('index', s, f'Missing index in slot {s}')
            self.assertIn('type',  s, f'Missing type in slot {s}')
            self.assertIn('name',  s, f'Missing name in slot {s}')
            self.assertIn('code',  s, f'Missing code in slot {s}')


if __name__ == '__main__':
    unittest.main()
