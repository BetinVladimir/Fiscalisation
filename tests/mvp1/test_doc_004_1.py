"""DOC-004.1 — COMPONENT_MAP.md exists with Mermaid diagram and table >= 10 rows."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MAP_FILE = ROOT / 'docs/MVP1/COMPONENT_MAP.md'


class TestDoc004(unittest.TestCase):
    def setUp(self):
        self.assertTrue(MAP_FILE.exists(), f'COMPONENT_MAP.md not found: {MAP_FILE}')
        self.text = MAP_FILE.read_text(encoding='utf-8')

    def test_has_mermaid_block(self):
        self.assertIn('```mermaid', self.text, 'Must contain a ```mermaid code block')

    def test_mermaid_has_fiscal_backend(self):
        self.assertIn('fiscal-backend', self.text,
                      'Mermaid diagram must reference fiscal-backend')

    def test_mermaid_has_mqtt(self):
        self.assertIn('MQTT', self.text, 'Mermaid diagram must reference MQTT')

    def test_table_row_count_gte_10(self):
        rows = [ln for ln in self.text.splitlines() if re.match(r'^\|\s*\d+\s*\|', ln)]
        self.assertGreaterEqual(len(rows), 10,
                                f'Need >= 10 table rows, found {len(rows)}')

    def test_table_has_timeout_column(self):
        self.assertIn('Таймаут', self.text, 'Table must have a Таймаут column')

    def test_table_has_idempotency_column(self):
        self.assertIn('Идемпотентность', self.text,
                      'Table must have an Идемпотентность column')

    def test_table_has_unknown_column(self):
        self.assertIn('UNKNOWN', self.text, 'Table must address UNKNOWN behavior')


if __name__ == '__main__':
    unittest.main()
