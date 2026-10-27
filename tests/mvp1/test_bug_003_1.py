"""BUG-003.1 — Simulated flag follows driver type."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG003_1(unittest.TestCase):
    def test_simulated_flag_follows_driver(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestOperationSimulatedFlagFollowsDriver$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestOperationSimulatedFlagFollowsDriver',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
