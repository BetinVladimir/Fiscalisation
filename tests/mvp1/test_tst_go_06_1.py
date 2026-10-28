"""TST-GO-06.1 — TestValidateRejectsUnknownAppEnv, TestLoadDefaultsAreSafe (BUG-009)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestTSTGO06(unittest.TestCase):
    def test_config_validation(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/config/',
             '-run', '^(TestValidateRejectsUnknownAppEnv'
                     '|TestLoadDefaultsAreSafe'
                     '|TestProdGuards'
                     '|TestValidateProdChecksApplyToStaging)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestValidateRejectsUnknownAppEnv',
                'TestLoadDefaultsAreSafe',
                'TestProdGuards',
                'TestValidateProdChecksApplyToStaging',
            ),
            minimum=4,
        )


if __name__ == '__main__':
    unittest.main()
