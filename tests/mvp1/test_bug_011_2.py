"""BUG-011.2 — HMAC: DenyAdminRoles, MaxTTL=24h, nbf, issuer checks."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestBUG0112(unittest.TestCase):
    def test_auth_token_validation(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/auth/',
             '-run', '^(TestHMACTokenCannotCarryAdminRoleInProd'
                     '|TestTokenWithTTLOverLimitRejected'
                     '|TestParse'
                     '|TestParseRejectsUnknownRole)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestHMACTokenCannotCarryAdminRoleInProd',
                'TestTokenWithTTLOverLimitRejected',
                'TestParse',
                'TestParseRejectsUnknownRole',
            ),
            minimum=4,
        )


if __name__ == '__main__':
    unittest.main()
