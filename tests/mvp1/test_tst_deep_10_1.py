"""TST-DEEP-10.1 — exactly-once 200 sales test file exists and executes (skips without infra)."""
import importlib.util
import os
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
E2E_TEST = ROOT / 'tests' / 'e2e' / 'test_exactly_once.py'


class TestTSTDEEP10_1(unittest.TestCase):
    def test_exactly_once_file_exists_with_test_class(self):
        self.assertTrue(E2E_TEST.exists(), f'test file missing: {E2E_TEST}')
        spec = importlib.util.spec_from_file_location('test_exactly_once', E2E_TEST)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        self.assertTrue(
            hasattr(mod, 'TestExactlyOnce200Sales'),
            'TestExactlyOnce200Sales class not found in test file',
        )
        cls = mod.TestExactlyOnce200Sales
        self.assertTrue(
            hasattr(cls, 'test_exactly_once_200_sales_5_duplicates'),
            'test_exactly_once_200_sales_5_duplicates method missing',
        )

    def test_exactly_once_runs_and_skips_without_infra(self):
        env_without_infra = {k: v for k, v in os.environ.items()
                             if k not in ('EXACTLY_ONCE_BASE_URL', 'EXACTLY_ONCE_FU_URL')}
        result = subprocess.run(
            [sys.executable, '-m', 'pytest', str(E2E_TEST), '-v', '--tb=short'],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=60,
            env=env_without_infra,
        )
        output = result.stdout + result.stderr
        self.assertIn('skipped', output.lower(),
                      'expected test to be skipped without infra env vars\n' + output[-2000:])


if __name__ == '__main__':
    unittest.main()
