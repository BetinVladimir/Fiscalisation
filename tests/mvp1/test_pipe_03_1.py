"""PIPE-03.1 — ARCH_21_DELIVERY_PIPELINE.md §7 contains a Факт calibration line."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
PIPELINE_DOC = ROOT / 'docs' / 'MVP1' / 'ARCH_21_DELIVERY_PIPELINE.md'


class TestPIPE03_1(unittest.TestCase):
    def test_pipeline_doc_exists(self):
        self.assertTrue(PIPELINE_DOC.exists(), f'Pipeline doc not found: {PIPELINE_DOC}')

    def test_section_7_contains_fact_line(self):
        text = PIPELINE_DOC.read_text(encoding='utf-8')
        # §7 header must be present
        self.assertIn('## 7.', text, 'Section §7 heading not found in pipeline doc')
        # Факт line must be present (in §7 area)
        self.assertIn('Факт', text, '"Факт" calibration line not found in pipeline doc')

    def test_fact_line_references_runs(self):
        text = PIPELINE_DOC.read_text(encoding='utf-8')
        # The Факт line must mention the runs directory
        self.assertIn('runs/', text, '"runs/" reference missing from Факт line')

    def test_section_7_before_section_8(self):
        text = PIPELINE_DOC.read_text(encoding='utf-8')
        idx7 = text.find('## 7.')
        idx8 = text.find('## 8.')
        idx_fact = text.find('Факт')
        self.assertGreater(idx7, 0, 'Section §7 not found')
        self.assertGreater(idx8, 0, 'Section §8 not found')
        # Факт must appear between §7 and §8
        self.assertGreater(idx_fact, idx7, '"Факт" appears before §7')
        self.assertLess(idx_fact, idx8, '"Факт" appears after §8 (not in §7)')


if __name__ == '__main__':
    unittest.main()
