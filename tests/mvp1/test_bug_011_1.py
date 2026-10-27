"""BUG-011.1 — TestHMACTokenCannotCarryAdminRoleInProd + TestTokenWithTTLOverLimitRejected."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG0111(unittest.TestCase):
    def test_hmac_admin_forbidden(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/auth/',
             '-run', '^(TestHMACTokenCannotCarryAdminRoleInProd|TestTokenWithTTLOverLimitRejected)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestHMACTokenCannotCarryAdminRoleInProd',
                'TestTokenWithTTLOverLimitRejected',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
