"""POS-09.1 — Secrets: no credentials.json, .jks, .keystore in git index."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS = ROOT / '../POS'
FISCAL = ROOT
CHECK_SCRIPT = FISCAL / 'scripts/check-no-secrets.sh'


class TestPos091(unittest.TestCase):
    def test_no_credential_files_in_git_index_pos(self):
        result = subprocess.run(
            ['git', 'ls-files'],
            cwd=str(POS),
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(result.returncode, 0,
                         'git ls-files failed in POS: ' + result.stderr)
        files = result.stdout
        import re
        forbidden = re.findall(r'(?:credentials\.json|\.jks|\.keystore)', files)
        self.assertEqual(
            forbidden, [],
            f'Forbidden secret files found in POS git index: {forbidden}',
        )

    def test_no_credential_files_in_git_index_fiscalisation(self):
        result = subprocess.run(
            ['git', 'ls-files'],
            cwd=str(FISCAL),
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(result.returncode, 0,
                         'git ls-files failed in Fiscalisation: ' + result.stderr)
        files = result.stdout
        import re
        forbidden = re.findall(r'(?:credentials\.json|\.jks|\.keystore)', files)
        self.assertEqual(
            forbidden, [],
            f'Forbidden secret files found in Fiscalisation git index: {forbidden}',
        )

    def test_check_no_secrets_script_exists(self):
        self.assertTrue(CHECK_SCRIPT.exists(),
                        f'check-no-secrets.sh not found at {CHECK_SCRIPT}')

    def test_check_no_secrets_script_is_executable_bash(self):
        content = CHECK_SCRIPT.read_text()
        self.assertIn('#!/usr/bin/env bash', content[:50],
                      'check-no-secrets.sh must have bash shebang')
        self.assertIn('secret', content.lower(),
                      'check-no-secrets.sh must scan for secrets')


if __name__ == '__main__':
    unittest.main()
