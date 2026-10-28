"""NAP-08.1-2 — Version management: InstalledVersions API + check-version-bump.sh."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'
SCRIPT = ROOT / 'scripts' / 'check-version-bump.sh'


def _run_go(pattern: str, pkg: str, timeout: int = 120) -> subprocess.CompletedProcess:
    return subprocess.run(
        ['go', 'test', '-v', '-count=1', '-run', pattern, pkg],
        cwd=str(BACKEND),
        capture_output=True,
        text=True,
        timeout=timeout,
    )


class TestNAP08VersionManagement(unittest.TestCase):

    def setUp(self):
        self._artifact = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())

    def _save(self, name: str, content: str):
        with open(os.path.join(self._artifact, name), 'w') as f:
            f.write(content)

    def test_installed_versions_go_test_passes(self):
        """TestInstalledVersionsInventory must exist in ./internal/api and pass."""
        r = _run_go('TestInstalledVersionsInventory', './internal/api/...')
        self._save('nap_08_installed_versions.txt', r.stdout + r.stderr)
        self.assertEqual(r.returncode, 0,
                         f'TestInstalledVersionsInventory failed:\n{r.stdout[-3000:]}\n{r.stderr[-500:]}')
        self.assertIn('PASS: TestInstalledVersionsInventory', r.stdout,
                      'TestInstalledVersionsInventory not found in Go test output')

    def test_version_bump_script_exists(self):
        """scripts/check-version-bump.sh must exist."""
        self.assertTrue(SCRIPT.exists(), f'check-version-bump.sh not found: {SCRIPT}')

    def test_version_bump_script_passes_on_clean_tree(self):
        """check-version-bump.sh must exit 0 on the current branch (no unresolved bumps)."""
        self.assertTrue(SCRIPT.exists(), f'check-version-bump.sh not found: {SCRIPT}')
        r = subprocess.run(
            ['bash', str(SCRIPT)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=60,
            env={**os.environ, 'BASE_BRANCH': 'HEAD~1'},
        )
        self._save('nap_08_version_bump.txt', r.stdout + r.stderr)
        self.assertEqual(r.returncode, 0,
                         f'check-version-bump.sh failed:\n{r.stdout}\n{r.stderr}')
        self.assertIn('VERSION-BUMP OK', r.stdout, 'Missing VERSION-BUMP OK marker')

    def test_version_go_file_exists(self):
        """fiscal-backend/internal/version/version.go must exist (VER-001 requires it)."""
        version_file = BACKEND / 'internal' / 'version' / 'version.go'
        self.assertTrue(version_file.exists(),
                        f'version.go not found at {version_file}')

    def test_supto_sensitive_paths_exists(self):
        """scripts/supto-sensitive-paths.txt must exist for VER-002 check."""
        sensitive = ROOT / 'scripts' / 'supto-sensitive-paths.txt'
        self.assertTrue(sensitive.exists(),
                        f'supto-sensitive-paths.txt not found at {sensitive}')
        text = sensitive.read_text()
        # Must list database migrations as sensitive
        self.assertIn('database/fiscal', text,
                      'supto-sensitive-paths.txt must include database/fiscal')


if __name__ == '__main__':
    unittest.main()
