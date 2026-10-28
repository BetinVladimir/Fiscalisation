"""SCOPE-06.1 — ARCH_13 extended with BeeFiscalApp App.tsx findings (BF-04)."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
ARCH13 = ROOT / 'docs/MVP1/ARCH_13_COMPONENTS_MINIPOS_ADMIN_ERP.md'

_REQUIRED_CARDS = ['### BF-04']
_MIN_CARD_COUNT = 29


class TestScope06(unittest.TestCase):
    def setUp(self):
        self.assertTrue(ARCH13.exists(), f'ARCH_13 not found: {ARCH13}')
        self.text = ARCH13.read_text(encoding='utf-8')

    def test_bf04_card_present(self):
        for card in _REQUIRED_CARDS:
            self.assertIn(card, self.text,
                          f'Required card header not found: {card!r}')

    def test_bf04_references_ble_app_instance(self):
        self.assertIn('bleAppInstance', self.text,
                      'BF-04 must reference bleAppInstance')
        self.assertIn('Date.now()', self.text,
                      'BF-04 must describe Date.now() usage')

    def test_bf04_references_source_file(self):
        self.assertIn('App.tsx:173', self.text,
                      'BF-04 must cite App.tsx line reference')

    def test_total_card_count_sufficient(self):
        count = len(re.findall(r'^### ', self.text, re.MULTILINE))
        self.assertGreaterEqual(count, _MIN_CARD_COUNT,
                                f'Expected >= {_MIN_CARD_COUNT} cards, found {count}')


if __name__ == '__main__':
    unittest.main()
