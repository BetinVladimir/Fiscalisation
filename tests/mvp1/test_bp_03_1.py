"""BP-03.1 — ADR-BLUEPAD-001.md exists with required sections."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestBP03_1(unittest.TestCase):
    def test_adr_exists_with_decision(self):
        f = ROOT / 'docs/MVP1/decisions/ADR-BLUEPAD-001.md'
        self.assertTrue(f.exists(), f'File not found: {f}')
        content = f.read_text()
        self.assertIn('## Решение', content,
                      f"Missing '## Решение' section in {f}")
        self.assertGreater(len(content), 100, 'File too short')
        self.assertIn('UNKNOWN', content,
                      "Decision must mention UNKNOWN state for connection failure")
        self.assertIn('0x24', content,
                      "Decision must reference 0x24 confirmation command")


if __name__ == '__main__':
    unittest.main()
