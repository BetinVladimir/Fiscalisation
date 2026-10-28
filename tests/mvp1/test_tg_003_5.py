"""TG-003.5 — HasDailyClockSync is O(1) via resource key; uses Sofia business date."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'


class TestTG003_5(unittest.TestCase):
    def test_has_daily_clock_sync_sofia_and_o1(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...', '-run',
             'TestHasDailyClockSyncUsesSofiaBusinessDate|TestClockSyncDurabilityAndBlocksWithoutSync',
             '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('TestHasDailyClockSyncUsesSofiaBusinessDate', r.stdout)
        self.assertIn('TestClockSyncDurabilityAndBlocksWithoutSync', r.stdout)


if __name__ == '__main__':
    unittest.main()
