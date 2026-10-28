"""TST-POS-01.1 — reconcileOperation URL: /reconcile (slash), not :reconcile (colon)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


def _run_jest(pattern: str, cwd: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        ['npx', 'jest', '--no-coverage', '--testPathPattern', pattern,
         '--json', '--outputFile', '/dev/stderr'],
        cwd=str(cwd),
        capture_output=True,
        text=True,
        timeout=120,
    )


class TestTstPos011(unittest.TestCase):
    def test_reconcile_url_uses_slash(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'infrastructure/runtime/__tests__/HttpFiscalApiClient',
             '--verbose'],
            cwd=str(POS_APP),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'jest rc=' + str(result.returncode) + '\n' + result.stdout[-3000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('reconcile', combined.lower(),
                      'reconcile test name not found in output')
        self.assertIn('PASS', combined,
                      'No PASS in jest output')
        # Ensure at least 2 tests ran
        self.assertRegex(combined, r'Tests:\s+\d+ passed',
                         'No test count found in jest output')


if __name__ == '__main__':
    unittest.main()
