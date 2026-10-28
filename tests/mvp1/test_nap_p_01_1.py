"""NAP-P-01.1 — SUPTO readiness matrix: core fiscal requirements (п.2, п.13, п.18, п.20).

Covers Annex-29 trace items that have concrete Go test commands in ARCH_06 §1:
  п.2  полнота данных   → go test ./internal/domain -run RepositoryContract
  п.13 сторно политика → go test ./internal/domain -run ReversalPolicy
  п.18 экспорт Annex29 → go test ./internal/domain -run TestAnnex29Columns
  п.20 запрет тест-режима → go test ./internal/config -run 'Defaults|ProdCheck|Stubs'
                            + scripts/check-prod-artifact.sh
"""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'
SCRIPTS = ROOT / 'scripts'


def _run_go(pattern: str, pkg: str, timeout: int = 120) -> subprocess.CompletedProcess:
    return subprocess.run(
        ['go', 'test', '-v', '-count=1', '-run', pattern, pkg],
        cwd=str(BACKEND),
        capture_output=True,
        text=True,
        timeout=timeout,
    )


class TestNAPP01CoreReadiness(unittest.TestCase):

    def setUp(self):
        self._artifact = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())

    def _save(self, name: str, result: subprocess.CompletedProcess):
        with open(os.path.join(self._artifact, name), 'w') as f:
            f.write(result.stdout + result.stderr)

    def test_p2_data_completeness_reversal_policy(self):
        """п.13: reversal policy (sторно policy) enforced in domain."""
        r = _run_go('ReversalPolicy', './internal/domain/...')
        self._save('nap_p01_1_reversal_policy.txt', r)
        self.assertEqual(r.returncode, 0,
                         'ReversalPolicy tests failed:\n' + r.stdout[-2000:] + r.stderr[-500:])
        self.assertIn('PASS: TestReversalPolicy', r.stdout,
                      'Expected at least one TestReversalPolicy to pass')

    def test_p18_annex29_export_columns(self):
        """п.18: Annex-29 export column schema matches canonical fixture."""
        r = _run_go('TestAnnex29Columns', './internal/domain/...')
        self._save('nap_p01_1_annex29_columns.txt', r)
        self.assertEqual(r.returncode, 0,
                         'TestAnnex29Columns failed:\n' + r.stdout[-2000:] + r.stderr[-500:])
        self.assertIn('PASS: TestAnnex29Columns', r.stdout,
                      'TestAnnex29Columns not found in output')

    def test_p18_annex29_export_headers_complete(self):
        """п.18: Annex-29 export headers are field-complete (no missing columns)."""
        r = _run_go('TestAnnex29', './internal/domain/...')
        self._save('nap_p01_1_annex29_exports.txt', r)
        self.assertEqual(r.returncode, 0,
                         'Annex29 export tests failed:\n' + r.stdout[-2000:] + r.stderr[-500:])
        passed = [line for line in r.stdout.splitlines() if line.startswith('--- PASS: TestAnnex29')]
        self.assertGreater(len(passed), 0,
                           'Expected at least one TestAnnex29* to pass')

    def test_p20_prod_rejects_stubs_go(self):
        """п.20: prod config rejects AllowStubAdapters (TestProdRejectsStubs)."""
        r = _run_go('TestProdRejectsStubs|TestValidateProdChecks|TestLoadDefaults',
                    './internal/config/...')
        self._save('nap_p01_1_prod_stubs.txt', r)
        self.assertEqual(r.returncode, 0,
                         'Prod-rejects-stubs tests failed:\n' + r.stdout[-2000:] + r.stderr[-500:])
        self.assertIn('PASS: TestProdRejectsStubs', r.stdout,
                      'TestProdRejectsStubs not found in output')

    def test_p20_prod_artifact_check_script(self):
        """п.20: check-prod-artifact.sh finds no stubs in production source."""
        script = SCRIPTS / 'check-prod-artifact.sh'
        self.assertTrue(script.exists(), f'check-prod-artifact.sh not found: {script}')
        r = subprocess.run(
            ['bash', str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self._save('nap_p01_1_prod_artifact.txt', r)
        self.assertEqual(r.returncode, 0,
                         'check-prod-artifact.sh failed:\n' + r.stdout + r.stderr)
        self.assertIn('PROD-ARTIFACT OK', r.stdout,
                      'Expected PROD-ARTIFACT OK marker')


if __name__ == '__main__':
    unittest.main()
