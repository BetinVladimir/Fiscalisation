"""DSY-04.1 — Daisy drop after cmd 48 returns UNKNOWN, not FAILED."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY04_1(unittest.TestCase):
    def test_daisy_drop_after_48(self):
        script = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests/run_native_saga.sh'
        if not script.exists():
            self.skipTest(f'Script not found: {script}')
        r = subprocess.run(
            ['bash', str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1000:]}',
        )
        self.assertIn(
            'receipt_saga: daisy_drop_after_48=pass', r.stdout,
            f'daisy_drop_after_48 marker not found in output:\n{r.stdout}',
        )


if __name__ == '__main__':
    unittest.main()
