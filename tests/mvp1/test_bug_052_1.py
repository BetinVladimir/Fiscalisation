"""BUG-052.1 — fiscal_reconcile_attempt table and nextReconcileAttemptNo repository."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG052_1(unittest.TestCase):
    def test_reconcile_attempt_table_in_schema(self):
        schema = POS / 'src/infrastructure/db/SQLiteClientImpl.ts'
        if not schema.exists():
            self.skipTest(f'{schema} not found')
        content = schema.read_text(encoding='utf-8')
        self.assertIn('fiscal_reconcile_attempt', content,
            'SQLiteClientImpl must define fiscal_reconcile_attempt table')
        self.assertIn('operation_id', content)
        self.assertIn('attempt_no', content)
        self.assertIn('key TEXT', content)

    def test_next_reconcile_attempt_no_jest(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'ReconcileAttempt', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}')
        import re
        m = re.search(r'Tests:\s+(\d+)\s+passed', r.stdout + r.stderr)
        count = int(m.group(1)) if m else 0
        self.assertGreaterEqual(count, 3, f'Expected at least 3 repository tests, got {count}')


if __name__ == '__main__':
    unittest.main()
