"""TST-DEEP-03.1 — crash replay after commit before response (test executes)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTDEEP03_1(unittest.TestCase):
    def test_crash_replay_after_commit_before_response(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/api/',
             '-run', '^(TestCrashReplayAfterCommitBeforeResponse'
                     '|TestCrashReplayPreservesExactlyOneSale)$',
             '-count=1', '-timeout=60s'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestCrashReplayAfterCommitBeforeResponse',
                'TestCrashReplayPreservesExactlyOneSale',
            ),
            minimum=2,
            timeout=120,
        )


if __name__ == '__main__':
    unittest.main()
