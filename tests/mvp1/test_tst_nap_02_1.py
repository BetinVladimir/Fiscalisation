"""TST-NAP-02.1 — prod artifact has no stubs/simulators; config rejects AllowStubAdapters in prod."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'scripts' / 'check-prod-artifact.sh'
BACKEND = ROOT / 'fiscal-backend'


class TestTSTNAP02(unittest.TestCase):
    def test_check_prod_artifact_script_passes(self):
        self.assertTrue(SCRIPT.exists(), f'script not found: {SCRIPT}')
        result = subprocess.run(
            ['bash', str(SCRIPT)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'check-prod-artifact.sh failed:\n' + result.stdout + result.stderr,
        )

    def test_go_prod_rejects_stubs(self):
        result = subprocess.run(
            ['go', 'test', './internal/config/...', '-run', 'TestProdRejectsStubs', '-v', '-count=1'],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'TestProdRejectsStubs failed:\n' + result.stdout + result.stderr,
        )
        self.assertIn('PASS: TestProdRejectsStubs', result.stdout,
                      'TestProdRejectsStubs not found in output:\n' + result.stdout)


if __name__ == '__main__':
    unittest.main()
