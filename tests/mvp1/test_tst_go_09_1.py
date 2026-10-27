"""TST-GO-09.1 — TestIdempotencyKeyReusableAfterPreSideEffect503 (BUG-012)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO09(unittest.TestCase):
    def test_idempotency_key_reusable_after_pre_side_effect_503(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/api/',
             '-run', '^(TestIdempotencyKeyReusableAfterPreSideEffect503'
                     '|TestIdempotencyCommittedReplayIsReturnedOnRetry)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestIdempotencyKeyReusableAfterPreSideEffect503',
                'TestIdempotencyCommittedReplayIsReturnedOnRetry',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
