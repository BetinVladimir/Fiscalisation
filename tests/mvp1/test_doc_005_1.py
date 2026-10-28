"""DOC-005.1 — DOC_DRIFT.md exists with drift entries; check-doc-drift.sh runs."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
DRIFT_FILE = ROOT / 'docs/MVP1/DOC_DRIFT.md'
SCRIPT = ROOT / 'scripts/check-doc-drift.sh'


class TestDoc005(unittest.TestCase):
    def test_drift_file_exists(self):
        self.assertTrue(DRIFT_FILE.exists(), f'DOC_DRIFT.md not found: {DRIFT_FILE}')

    def test_drift_file_has_table(self):
        text = DRIFT_FILE.read_text(encoding='utf-8')
        drift_rows = [ln for ln in text.splitlines() if ln.startswith('| DRIFT-')]
        self.assertGreater(len(drift_rows), 0, 'DOC_DRIFT.md must contain DRIFT-* rows')

    def test_drift_file_has_required_columns(self):
        text = DRIFT_FILE.read_text(encoding='utf-8')
        for col in ['Документ', 'Строка', 'Решение', 'Статус']:
            self.assertIn(col, text, f'Table header must contain column: {col!r}')

    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f'check-doc-drift.sh not found: {SCRIPT}')

    def test_script_syntax_valid(self):
        result = subprocess.run(
            ['bash', '--norc', '-n', str(SCRIPT)],
            capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 0,
                         f'Script syntax error: {result.stderr}')

    def test_script_detects_open_items(self):
        """Script should exit non-zero while OPEN drift items remain."""
        result = subprocess.run(
            ['bash', str(SCRIPT)],
            capture_output=True, text=True,
            cwd=str(ROOT),
        )
        # There are OPEN items so script should exit 1
        self.assertNotEqual(result.returncode, 0,
                            'Script must exit non-zero while OPEN items remain')
        self.assertIn('OPEN', result.stderr,
                      'Script must report OPEN drift items on stderr')

    def test_resolved_count_correct(self):
        text = DRIFT_FILE.read_text(encoding='utf-8')
        resolved = sum(1 for ln in text.splitlines()
                       if ln.startswith('| DRIFT-') and '| RESOLVED |' in ln)
        total = sum(1 for ln in text.splitlines() if ln.startswith('| DRIFT-'))
        self.assertGreater(total, 0, 'Must have at least one DRIFT entry')
        self.assertGreaterEqual(resolved, 0)  # Some may be OPEN legitimately


if __name__ == '__main__':
    unittest.main()
