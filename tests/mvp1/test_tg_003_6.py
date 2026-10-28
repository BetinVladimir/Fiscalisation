"""TG-003.6 — TestClockDriftOver2hBlocksSales (2h01m→BLOCK), TestClockDrift5minWarnsOnly."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'


class TestTG003_6(unittest.TestCase):
    def test_clock_drift_thresholds(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...', '-run',
             'TestClockDriftOver2hBlocksSales|TestClockDrift5minWarnsOnly',
             '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('TestClockDriftOver2hBlocksSales', r.stdout)
        self.assertIn('TestClockDrift5minWarnsOnly', r.stdout)


if __name__ == '__main__':
    unittest.main()
