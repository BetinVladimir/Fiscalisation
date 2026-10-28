"""BUG-006.3 — check-lint-rules.sh (SCR-17) bans MemoryReceiptSagaStore re-introduction."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestBUG006_3(unittest.TestCase):
    def test_lint_rules_script_exists(self):
        f = ROOT / 'scripts/check-lint-rules.sh'
        self.assertTrue(f.exists(), f'check-lint-rules.sh not found: {f}')
        content = f.read_text()
        self.assertIn('MemoryReceiptSagaStore', content,
                      'Script must check for MemoryReceiptSagaStore')

    def test_no_saga_store_in_production_code(self):
        r = subprocess.run(
            ['bash', 'scripts/check-lint-rules.sh'],
            cwd=str(ROOT), capture_output=True, text=True, timeout=60)
        self.assertEqual(r.returncode, 0,
                         f'lint check failed:\n{r.stdout}\n{r.stderr}')

    def test_saga_grep_empty(self):
        r = subprocess.run(
            ['grep', '-rn', 'MemoryReceiptSagaStore',
             'fiscal-backend/internal', '--include=*.go'],
            cwd=str(ROOT), capture_output=True, text=True, timeout=30)
        self.assertEqual(r.returncode, 1,
                         f'MemoryReceiptSagaStore still found:\n{r.stdout}')


if __name__ == '__main__':
    unittest.main()
