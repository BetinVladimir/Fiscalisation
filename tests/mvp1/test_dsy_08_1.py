"""DSY-08.1 — DAISY_CURRENCY.md contains cmd= instruction and required fields."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY08_1(unittest.TestCase):
    def test_daisy_currency_doc_has_cmd(self):
        f = ROOT / 'docs/vendor/DAISY_CURRENCY.md'
        self.assertTrue(f.exists(), f'File not found: {f}')
        content = f.read_text()
        self.assertIn('cmd=', content, "File must contain 'cmd=' instruction")
        self.assertGreater(len(content), 200, 'File too short')

    def test_daisy_currency_doc_has_modes(self):
        f = ROOT / 'docs/vendor/DAISY_CURRENCY.md'
        content = f.read_text()
        # Must document BGN (2) and EUR (3)
        self.assertIn('BGN', content, "Missing BGN currency mode")
        self.assertIn('EUR', content, "Missing EUR currency mode")
        # Must reference mode codes
        self.assertIn('2', content)
        self.assertIn('3', content)

    def test_daisy_currency_doc_references_not_ready(self):
        f = ROOT / 'docs/vendor/DAISY_CURRENCY.md'
        content = f.read_text()
        # Must document that non-EUR makes device not ready
        self.assertTrue(
            'NOT_READY' in content or 'not ready' in content.lower(),
            "Must document NOT_READY for non-EUR mode",
        )


if __name__ == '__main__':
    unittest.main()
