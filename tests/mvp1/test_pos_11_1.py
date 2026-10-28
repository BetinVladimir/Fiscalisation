"""POS-11.1 — UNP timing: POS_SALE_FLOWS.md with scenario/UNP-moment/component/test table."""
import re
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
FLOWS_DOC = ROOT / 'docs/MVP1/POS_SALE_FLOWS.md'


class TestPos111(unittest.TestCase):
    def setUp(self):
        self.assertTrue(FLOWS_DOC.exists(),
                        f'POS_SALE_FLOWS.md not found at {FLOWS_DOC}')
        self.content = FLOWS_DOC.read_text()

    def test_document_has_table_header_with_four_columns(self):
        # Must have at minimum: Scenario | UNP Moment | Component | Test (4 cols)
        lines = [l.strip() for l in self.content.splitlines() if '|' in l]
        self.assertGreater(len(lines), 0, 'No table rows found in POS_SALE_FLOWS.md')
        for line in lines[:5]:
            if line.startswith('|') and not re.match(r'^\|[-| ]+\|$', line):
                cols = [c.strip() for c in line.split('|') if c.strip()]
                self.assertGreaterEqual(
                    len(cols), 4,
                    f'Table row has <4 columns: {line!r}',
                )

    def test_required_keywords_present(self):
        for kw in ['UNP', 'Scenario', 'Component', 'Test', 'openSaleWithLine']:
            self.assertIn(kw, self.content,
                          f'Required keyword {kw!r} missing from POS_SALE_FLOWS.md')

    def test_no_empty_cells(self):
        rows = [l.strip() for l in self.content.splitlines()
                if l.strip().startswith('|') and '|' in l[1:]]
        for row in rows:
            if re.match(r'^\|[-| ]+\|$', row):
                continue  # separator row
            cells = [c.strip() for c in row.split('|') if c != '']
            for cell in cells:
                self.assertNotEqual(
                    cell, '',
                    f'Empty cell found in table row: {row!r}',
                )

    def test_table_has_at_least_three_scenario_rows(self):
        data_rows = [
            l.strip() for l in self.content.splitlines()
            if l.strip().startswith('|')
            and not re.match(r'^\|[-| ]+\|$', l.strip())
            and 'Scenario' not in l
            and 'scenario' not in l.lower().replace('scenario', '')
        ]
        # Count rows that look like data (not separator, not header)
        scenario_rows = [r for r in data_rows if '|' in r[1:]]
        self.assertGreaterEqual(
            len(scenario_rows), 3,
            f'Expected ≥3 scenario rows, found {len(scenario_rows)}',
        )


if __name__ == '__main__':
    unittest.main()
