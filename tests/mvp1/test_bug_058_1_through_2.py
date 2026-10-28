"""BUG-058.1-2 — acceptance: policy lookup error returns 503, not fail-open."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests

POS_GATEWAY = ROOT.parent / 'POS' / 'pos-gateway'


class TestBUG0581_2(unittest.TestCase):
    def test_fails_closed_on_policy_lookup_error(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/handler/',
             '-run', '^TestSubmit_failsClosedOnPolicyLookupError$',
             '-count=1'],
            root=POS_GATEWAY,
            cwd='.',
            required_tests=('TestSubmit_failsClosedOnPolicyLookupError',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
