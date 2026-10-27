"""BUG-011.3 — acceptance: полный пакет auth зелёный."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG0113(unittest.TestCase):
    def test_auth_full_suite(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/auth/',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestHMACTokenCannotCarryAdminRoleInProd',
                'TestTokenWithTTLOverLimitRejected',
                'TestRBACLeastPrivilege',
                'TestParse',
            ),
            minimum=4,
        )


if __name__ == '__main__':
    unittest.main()
