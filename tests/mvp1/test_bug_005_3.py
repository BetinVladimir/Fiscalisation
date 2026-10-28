"""BUG-005.3 — business_date uses Europe/Sofia (polночь/DST boundary)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestBUG005_3(unittest.TestCase):
    def test_sofia_business_date_timezone(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestHasDailyClockSyncUsesSofiaBusinessDate$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestHasDailyClockSyncUsesSofiaBusinessDate',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
