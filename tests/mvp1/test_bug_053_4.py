"""BUG-053.4 — LocalSuptoJournal schema has separate fiscal_number, fiscal_reference columns."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG053_4(unittest.TestCase):
    def test_supto_schema_has_separate_fiscal_columns(self):
        schema = POS / 'src/infrastructure/db/SQLiteClientImpl.ts'
        if not schema.exists():
            self.skipTest(f'{schema} not found')
        content = schema.read_text(encoding='utf-8')
        self.assertIn('fiscal_number', content,
            'SQLite supto journal must have fiscal_number column')
        self.assertIn('fiscal_reference', content,
            'SQLite supto journal must have fiscal_reference column')

    def test_supto_journal_interface_has_separate_fields(self):
        journal = POS / 'src/infrastructure/supto/LocalSuptoJournal.ts'
        if not journal.exists():
            self.skipTest(f'{journal} not found')
        content = journal.read_text(encoding='utf-8')
        self.assertIn('fiscalNumber?:number', content,
            'SuptoJournalEvent must have fiscalNumber field')
        self.assertIn('fiscalReference?:string', content,
            'SuptoJournalEvent must have fiscalReference field')

    def test_supto_journal_tests_pass(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'LocalSuptoJournal', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}')


if __name__ == '__main__':
    unittest.main()
