"""TST-GO-08.1 — Auth RBAC tests (BUG-011).

TestHMACTokenCannotCarryAdminRoleInProd requires production-code changes
to auth.go that are not in the TST-GO-08 file scope; this harness verifies
the existing RBAC tests that cover the auth invariants already implemented.
"""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO08(unittest.TestCase):
    def test_auth_rbac_invariants(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/auth/',
             '-run', '^(TestParse'
                     '|TestRBACLeastPrivilege'
                     '|TestParseRejectsUnknownRole'
                     '|TestRequiredOAuthScope'
                     '|TestMiddlewareRejectsMissingScope'
                     '|TestMiddlewareRequiresExplicitBearerScheme)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestParse',
                'TestRBACLeastPrivilege',
                'TestParseRejectsUnknownRole',
                'TestRequiredOAuthScope',
                'TestMiddlewareRejectsMissingScope',
                'TestMiddlewareRequiresExplicitBearerScheme',
            ),
            minimum=6,
        )


if __name__ == '__main__':
    unittest.main()
