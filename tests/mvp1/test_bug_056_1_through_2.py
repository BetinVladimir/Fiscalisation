"""BUG-056.1-2 — acceptance: check projection uses payload totals, not round($5/1.2,4)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests

POS_GATEWAY = ROOT.parent / 'POS' / 'pos-gateway'


class TestBUG0561_2(unittest.TestCase):
    def test_projects_totals_from_payload(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/handler/',
             '-run', '^TestSubmit_projectsTotalsFromPayload$|^TestSubmit_doesNotAssume20PercentVAT$',
             '-count=1'],
            root=POS_GATEWAY,
            cwd='.',
            required_tests=(
                'TestSubmit_projectsTotalsFromPayload',
                'TestSubmit_doesNotAssume20PercentVAT',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
