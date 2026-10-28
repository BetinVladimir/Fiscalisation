"""PERSIST-01.1 — PostgreSQL schema: unp_allocations UNIQUE(tenant_id,unp) constraint test."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
FISCAL_BACKEND = ROOT / 'fiscal-backend'

REQUIRED_TEST = 'TestUNPUniqueConstraint'


class TestPERSIST01_1(unittest.TestCase):
    def test_unp_unique_constraint_schema_test_exists_and_compiles(self):
        result = subprocess.run(
            ['go', 'test', '-run', REQUIRED_TEST, '-v', '-count=1',
             './internal/persistence/'],
            cwd=str(FISCAL_BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f'go test rc={result.returncode}\n{output[-3000:]}',
        )
        self.assertIn(
            REQUIRED_TEST, output,
            f'Expected test {REQUIRED_TEST!r} in output; got:\n{output[-2000:]}',
        )
        self.assertNotIn(
            f'--- FAIL: {REQUIRED_TEST}', output,
            f'Test {REQUIRED_TEST!r} reported FAIL:\n{output[-2000:]}',
        )

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'native-output'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'persist_01_1.stdout.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
