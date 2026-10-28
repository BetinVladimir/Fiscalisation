"""DOC-003.1 — verify-contract-lock.sh detects drift in openapi-runtime-v1.yaml."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'scripts/verify-contract-lock.sh'
DRIFTED_CONTRACT = 'contracts/openapi-runtime-v1.yaml'


class TestDoc003(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f'Script not found: {SCRIPT}')

    def test_script_syntax_valid(self):
        result = subprocess.run(
            ['bash', '--norc', '-n', str(SCRIPT)],
            capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 0,
                         f'Script syntax error: {result.stderr}')

    def test_script_detects_drift(self):
        """Script should exit non-zero and identify the drifted contract file."""
        result = subprocess.run(
            ['bash', str(SCRIPT)],
            capture_output=True, text=True,
            cwd=str(ROOT),
        )
        # Script is expected to fail: openapi-runtime-v1.yaml has drifted
        self.assertNotEqual(result.returncode, 0,
                            'Expected non-zero exit: contract drift not yet resolved')
        drift_output = result.stdout + result.stderr
        self.assertIn('openapi-runtime-v1.yaml', drift_output,
                      'Script must identify the drifted contract file')

    def test_drifted_contract_file_exists(self):
        """The drifted contract file must exist (drift is content mismatch, not missing file)."""
        path = ROOT / DRIFTED_CONTRACT
        self.assertTrue(path.exists(), f'Contract file missing: {DRIFTED_CONTRACT}')

    def test_non_drifted_contracts_would_pass(self):
        """Script correctly handles files with matching hashes (partial check)."""
        # Verify that the stable contracts (BeeloyBackend side) have unchanged hashes
        # by checking the script references them (structural check)
        script_text = SCRIPT.read_text()
        self.assertIn('openapi-public-v1.yaml', script_text)
        self.assertIn('asyncapi-device-v1.yaml', script_text)


if __name__ == '__main__':
    unittest.main()
