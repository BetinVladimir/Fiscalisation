"""SCOPE-04.1 — ARCH_13 extended with erp-connector applier findings (EC-08)."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
ARCH13 = ROOT / 'docs/MVP1/ARCH_13_COMPONENTS_MINIPOS_ADMIN_ERP.md'

_REQUIRED_CARDS = ['### EC-08']
_MIN_CARD_COUNT = 29


class TestScope04(unittest.TestCase):
    def setUp(self):
        self.assertTrue(ARCH13.exists(), f'ARCH_13 not found: {ARCH13}')
        self.text = ARCH13.read_text(encoding='utf-8')

    def test_ec08_card_present(self):
        for card in _REQUIRED_CARDS:
            self.assertIn(card, self.text,
                          f'Required card header not found: {card!r}')

    def test_ec08_references_applier(self):
        self.assertIn('ApplyMessage', self.text,
                      'EC-08 must reference ApplyMessage')
        self.assertIn('messageID', self.text,
                      'EC-08 must describe messageID deduplication issue')

    def test_ec08_references_source_file(self):
        self.assertIn('pg.go:25', self.text,
                      'EC-08 must cite applier/pg.go line reference')

    def test_total_card_count_sufficient(self):
        count = len(re.findall(r'^### ', self.text, re.MULTILINE))
        self.assertGreaterEqual(count, _MIN_CARD_COUNT,
                                f'Expected >= {_MIN_CARD_COUNT} cards, found {count}')


if __name__ == '__main__':
    unittest.main()
