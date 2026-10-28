"""DSY-05.2 — Native saga binary passes all 7 storno matrix cases."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY05_2(unittest.TestCase):
    def test_storno_matrix_7_cases(self):
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
            'storno_matrix pass=7', r.stdout,
            f'storno_matrix pass=7 marker not found in output:\n{r.stdout}',
        )


if __name__ == '__main__':
    unittest.main()
