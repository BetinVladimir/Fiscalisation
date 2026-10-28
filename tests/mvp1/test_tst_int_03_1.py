"""TST-INT-03.1 — integration test file exists and skips without infra."""
import importlib.util
import os
import sys
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
INT_TEST = ROOT / 'tests' / 'e2e' / 'integration' / 'test_tst_int_03.py'


class TestTSTINT03_1(unittest.TestCase):
    def test_integration_file_exists_with_test(self):
        self.assertTrue(INT_TEST.exists(), f'integration test missing: {INT_TEST}')
        spec = importlib.util.spec_from_file_location('test_tst_int_03', INT_TEST)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        self.assertTrue(any(
            name.startswith('TestTSTINT03') for name in dir(mod)
        ), 'TestTSTINT03* class not found')

    def test_integration_skips_without_infra(self):
        env = {k: v for k, v in os.environ.items()
               if k not in ('INTEGRATION_BASE_URL', 'INTEGRATION_COMPOSE_DIR')}
        r = subprocess.run(
            [sys.executable, '-m', 'pytest', str(INT_TEST), '-v', '--tb=short'],
            cwd=str(ROOT), capture_output=True, text=True, timeout=60, env=env,
        )
        self.assertIn('skipped', (r.stdout + r.stderr).lower(),
                      'test must skip without infra\n' + r.stdout[-2000:])


if __name__ == '__main__':
    unittest.main()
