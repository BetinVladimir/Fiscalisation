"""TST-NAP-01.1 — Annex 29 column JSON loaded and TestAnnex29Columns passes."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'
FIXTURE = BACKEND / 'internal' / 'domain' / 'testdata' / 'annex29_columns.json'


class TestTSTNAP01(unittest.TestCase):
    def test_fixture_exists_and_valid(self):
        import json
        self.assertTrue(FIXTURE.exists(), f'annex29_columns.json not found at {FIXTURE}')
        data = json.loads(FIXTURE.read_text(encoding='utf-8'))
        self.assertIn('tables', data, 'missing "tables" key')
        self.assertIn('total_columns', data, 'missing "total_columns" key')
        for tbl in ('18.1', '18.2', '18.3', '18.4', '18.5', '18.9'):
            self.assertIn(tbl, data['tables'], f'table {tbl} missing from fixture')
            self.assertGreater(len(data['tables'][tbl]), 0, f'table {tbl} has no columns')
        self.assertGreaterEqual(data['total_columns'], 10, 'total_columns too small')

    def test_go_annex29_columns(self):
        result = subprocess.run(
            ['go', 'test', './internal/domain/...', '-run', 'TestAnnex29Columns', '-v', '-count=1'],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'TestAnnex29Columns failed:\n' + result.stdout + result.stderr,
        )
        self.assertIn('PASS: TestAnnex29Columns', result.stdout,
                      'TestAnnex29Columns not in output:\n' + result.stdout)


if __name__ == '__main__':
    unittest.main()
