"""DOC-001.1 — check-doc-invariants.sh infrastructure: function list >= 20, script is valid."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
FUNC_LIST = ROOT / 'docs/MVP1/doc-required-functions.txt'
SCRIPT = ROOT / 'scripts/check-doc-invariants.sh'


class TestDoc001(unittest.TestCase):
    def test_function_list_exists(self):
        self.assertTrue(FUNC_LIST.exists(), f'Function list not found: {FUNC_LIST}')

    def test_function_list_has_20_entries(self):
        entries = [
            ln for ln in FUNC_LIST.read_text().splitlines()
            if ln.strip() and not ln.strip().startswith('#')
        ]
        self.assertGreaterEqual(len(entries), 20,
                                f'Need >= 20 function entries, found {len(entries)}')

    def test_function_list_entries_have_two_fields(self):
        entries = [
            ln for ln in FUNC_LIST.read_text().splitlines()
            if ln.strip() and not ln.strip().startswith('#')
        ]
        for entry in entries:
            parts = entry.split()
            self.assertEqual(len(parts), 2,
                             f'Entry must be "filepath funcname", got: {entry!r}')

    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f'Script not found: {SCRIPT}')

    def test_script_syntax_valid(self):
        result = subprocess.run(
            ['bash', '--norc', '-n', str(SCRIPT)],
            capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 0,
                         f'Script syntax error: {result.stderr}')

    def test_script_references_invariants_pattern(self):
        text = SCRIPT.read_text()
        self.assertIn('Invariants:', text,
                      'Script must search for "Invariants:" pattern')

    def test_script_runs_and_reports_missing(self):
        """Script must exit non-zero (functions lack Invariants: blocks) and report findings."""
        result = subprocess.run(
            ['bash', str(SCRIPT)],
            capture_output=True, text=True,
            cwd=str(ROOT),
        )
        # Script should exit 1 because source files lack Invariants: blocks
        stderr = result.stderr
        self.assertIn('missing Invariants', stderr,
                      f'Script must report missing Invariants: blocks; stderr={stderr!r}')
        # Each NO_INVARIANTS line must show a real file path
        lines = [l for l in stderr.splitlines() if l.startswith('NO_INVARIANTS')]
        self.assertGreater(len(lines), 0,
                           'Script must identify at least one missing Invariants: block')


if __name__ == '__main__':
    unittest.main()
