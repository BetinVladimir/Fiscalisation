"""BUG-010.1-2 — /readyz 503 on DB outage; /livez always 200."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestBUG010_1_2(unittest.TestCase):
    def test_readyz_liveness_separation(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/api/',
             '-run', '^(TestReadyzFailsWhenRepositoryUnavailable|TestReadyzOKWhenRepositoryHealthy)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestReadyzFailsWhenRepositoryUnavailable',
                'TestReadyzOKWhenRepositoryHealthy',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
