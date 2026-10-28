"""TG-003.4 — ClockDriftMax=2h, ClockDriftWarn=5min; BusinessDate uses Europe/Sofia."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'


class TestTG003_4(unittest.TestCase):
    def test_clock_drift_constants_and_business_date(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...', '-run',
             'TestClockDriftConstants|TestBusinessDateUsesEuropeSofia|TestBusinessDateMidnightSofia',
             '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('TestClockDriftConstants', r.stdout)
        self.assertIn('TestBusinessDateUsesEuropeSofia', r.stdout)
        self.assertIn('TestBusinessDateMidnightSofia', r.stdout)


if __name__ == '__main__':
    unittest.main()
