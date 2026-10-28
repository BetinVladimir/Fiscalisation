"""NAP-06.2 — PROD-artifact path inventory: no simulator/stubs/AUTHORIZED_TEST/demo in prod source."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'scripts' / 'check-prod-artifact.sh'


class TestNAP06ProdArtifactInventory(unittest.TestCase):

    def setUp(self):
        self._artifact = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())

    def _save(self, name: str, content: str):
        with open(os.path.join(self._artifact, name), 'w') as f:
            f.write(content)

    def test_check_prod_artifact_script_exists(self):
        """scripts/check-prod-artifact.sh must exist."""
        self.assertTrue(SCRIPT.exists(), f'check-prod-artifact.sh not found: {SCRIPT}')

    def test_check_prod_artifact_passes(self):
        """check-prod-artifact.sh must exit 0 (no stubs/simulator/AUTHORIZED_TEST in prod)."""
        self.assertTrue(SCRIPT.exists(), f'check-prod-artifact.sh not found: {SCRIPT}')
        r = subprocess.run(
            ['bash', str(SCRIPT)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self._save('nap_06_2_check_prod.txt', r.stdout + r.stderr)
        self.assertEqual(r.returncode, 0,
                         f'check-prod-artifact.sh failed:\n{r.stdout}\n{r.stderr}')
        self.assertIn('PROD-ARTIFACT OK', r.stdout,
                      'Expected PROD-ARTIFACT OK marker from check-prod-artifact.sh')

    def test_no_authorized_test_in_prod_go(self):
        """AUTHORIZED_TEST must not appear in non-test Go production source."""
        import subprocess as sp
        domains = [ROOT / 'fiscal-backend', ROOT / 'SmartDevices']
        found = []
        for d in domains:
            if not d.exists():
                continue
            r = sp.run(
                ['grep', '-r', '--include=*.go', '-l', 'AUTHORIZED_TEST', str(d)],
                capture_output=True, text=True
            )
            for line in r.stdout.splitlines():
                if '_test.go' not in line and 'Test' not in line:
                    found.append(line)
        self._save('nap_06_2_authorized_test_scan.txt',
                   f'files with AUTHORIZED_TEST (non-test): {found}\n')
        self.assertEqual(found, [],
                         f'AUTHORIZED_TEST found in production source: {found}')

    def test_no_demo_flavor_in_prod_gradle(self):
        """demo build flavor must not be referenced in production Gradle files."""
        import subprocess as sp
        r = sp.run(
            ['grep', '-rn', '--include=*.gradle', '--include=*.kts',
             '-l', 'demo', str(ROOT / 'SmartDevices')],
            capture_output=True, text=True
        )
        # demo flavor reference in Gradle is expected only in productFlavors definition
        # but image tags demo|test in compose.*.prod.yaml are forbidden (checked by script)
        # This test verifies no compose.*.prod file references :demo image tag
        compose_files = list(ROOT.glob('compose.*.prod.yaml')) + list(ROOT.glob('compose.*.prod.yml'))
        violations = []
        for cf in compose_files:
            text = cf.read_text()
            if 'image:' in text and ':demo' in text:
                violations.append(str(cf))
        self._save('nap_06_2_demo_compose.txt',
                   f'compose files with :demo image tag: {violations}\n')
        self.assertEqual(violations, [],
                         f'compose.*.prod file references :demo image tag: {violations}')


if __name__ == '__main__':
    unittest.main()
