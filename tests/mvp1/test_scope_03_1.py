"""SCOPE-03.1 — ARCH_13 extended with BeeMiniPOS App.tsx findings (BF-03)."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
ARCH13 = ROOT / 'docs/MVP1/ARCH_13_COMPONENTS_MINIPOS_ADMIN_ERP.md'

_REQUIRED_CARDS = ['### BF-03']
_MIN_CARD_COUNT = 29


class TestScope03(unittest.TestCase):
    def setUp(self):
        self.assertTrue(ARCH13.exists(), f'ARCH_13 not found: {ARCH13}')
        self.text = ARCH13.read_text(encoding='utf-8')

    def test_bf03_card_present(self):
        for card in _REQUIRED_CARDS:
            self.assertIn(card, self.text,
                          f'Required card header not found: {card!r}')

    def test_bf03_references_async_storage(self):
        self.assertIn('AsyncStorage', self.text,
                      'BF-03 must reference AsyncStorage')
        self.assertIn('SecureStore', self.text,
                      'BF-03 must reference SecureStore as the fix')

    def test_bf03_references_source_file(self):
        self.assertIn('App.tsx:400', self.text,
                      'BF-03 must cite App.tsx line reference')

    def test_total_card_count_sufficient(self):
        count = len(re.findall(r'^### ', self.text, re.MULTILINE))
        self.assertGreaterEqual(count, _MIN_CARD_COUNT,
                                f'Expected >= {_MIN_CARD_COUNT} cards, found {count}')


if __name__ == '__main__':
    unittest.main()
