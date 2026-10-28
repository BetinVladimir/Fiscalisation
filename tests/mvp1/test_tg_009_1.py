"""TG-009.1 — STORNO_RULES.md exists with required fields."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG009_1(unittest.TestCase):
    def test_storno_rules_md_exists_and_has_rows(self):
        rules_path = ROOT / 'docs' / 'SUPTO' / 'STORNO_RULES.md'
        self.assertTrue(rules_path.exists(), f'STORNO_RULES.md not found at {rules_path}')
        content = rules_path.read_text(encoding='utf-8')
        # Must have the table header and at least one SR row with алинея reference
        self.assertIn('| SR-', content, 'No SR rule rows found')
        self.assertIn('чл. 31', content, 'No Art. 31 reference found')
        row_count = sum(1 for line in content.splitlines() if '| SR-' in line)
        self.assertGreaterEqual(row_count, 4, f'Expected >= 4 rows, got {row_count}')


if __name__ == '__main__':
    unittest.main()
