"""BUG-062.1-2 — UNP golden validation in Go (TestUNPGolden)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'


class TestBUG062_12(unittest.TestCase):
    def test_unp_golden_go(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...', '-run', 'TestUNPGolden', '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f'go test rc={r.returncode}\n{r.stdout}\n{r.stderr}',
        )
        self.assertIn('PASS: TestUNPGolden', r.stdout,
            'TestUNPGolden must be in passing tests')
        # Verify both valid and invalid sub-tests ran
        self.assertIn('valid/', r.stdout, 'Expected valid sub-tests')
        self.assertIn('invalid/', r.stdout, 'Expected invalid sub-tests')

    def test_golden_file_has_valid_and_invalid(self):
        import json
        golden = ROOT / 'contracts' / 'bg-unp-v1-golden.json'
        self.assertTrue(golden.exists(), f'{golden} not found')
        data = json.loads(golden.read_text(encoding='utf-8'))
        self.assertGreater(len(data.get('valid', [])), 0,
            'golden file must have valid entries')
        self.assertGreater(len(data.get('invalid', [])), 0,
            'golden file must have invalid entries')


if __name__ == '__main__':
    unittest.main()
