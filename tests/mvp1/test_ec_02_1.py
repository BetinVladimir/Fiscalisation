"""EC-02 — classify() UNKNOWN≠FAILED outcome tests."""
import subprocess, unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
ERP = ROOT / '..' / 'POS' / 'erp-connector'

class TestEC02_1(unittest.TestCase):
    def test_classify_outcome_tests(self):
        r = subprocess.run(
            ['go', 'test', './internal/orchestration', '-run', 'Classify', '-count=1', '-v'],
            cwd=str(ERP), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0, f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('PASS', r.stdout)

if __name__ == '__main__':
    unittest.main()
