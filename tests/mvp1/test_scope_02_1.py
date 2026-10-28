"""SCOPE-02.1 — ARCH_13 extended with minipos domain/service.go findings (MP-08)."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
ARCH13 = ROOT / 'docs/MVP1/ARCH_13_COMPONENTS_MINIPOS_ADMIN_ERP.md'

_REQUIRED_CARDS = ['### MP-08']
_MIN_CARD_COUNT = 29  # 24 existing + 5 new (MP-08, BF-03, EC-08, PA-04, BF-04)


class TestScope02(unittest.TestCase):
    def setUp(self):
        self.assertTrue(ARCH13.exists(), f'ARCH_13 not found: {ARCH13}')
        self.text = ARCH13.read_text(encoding='utf-8')

    def test_mp08_card_present(self):
        for card in _REQUIRED_CARDS:
            self.assertIn(card, self.text,
                          f'Required card header not found: {card!r}')

    def test_mp08_references_normalize_tax_rate(self):
        self.assertIn('normalizeTaxRate', self.text,
                      'MP-08 must reference normalizeTaxRate function')
        self.assertIn('ParseFloat', self.text,
                      'MP-08 must describe ParseFloat usage')

    def test_mp08_references_source_file(self):
        self.assertIn('service.go:771', self.text,
                      'MP-08 must cite service.go line reference')

    def test_total_card_count_sufficient(self):
        count = len(re.findall(r'^### ', self.text, re.MULTILINE))
        self.assertGreaterEqual(count, _MIN_CARD_COUNT,
                                f'Expected >= {_MIN_CARD_COUNT} cards, found {count}')


if __name__ == '__main__':
    unittest.main()
