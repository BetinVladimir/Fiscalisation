"""EC-01.1 — inbox state machine migration file exists with required columns."""
import unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent

class TestEC01_1(unittest.TestCase):
    def test_migration_file_exists(self):
        migrations_dir = ROOT / '..' / 'POS' / 'erp-connector' / 'migrations'
        sql_files = list(migrations_dir.glob('*inbox_state_machine*')) if migrations_dir.exists() else []
        self.assertGreater(len(sql_files), 0, f'No inbox_state_machine migration in {migrations_dir}')
        content = sql_files[0].read_text()
        for col in ('status', 'attempt', 'locked_until', 'result_published'):
            self.assertIn(col, content, f'Column {col!r} missing from migration')
        self.assertIn('erp_fiscal_result_outbox', content, 'Outbox table missing')

if __name__ == '__main__':
    unittest.main()
