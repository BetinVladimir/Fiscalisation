"""S3FIX-01.1 — DurableStorage: storage error stops I/O, UNP 7 digits, restart no reuse."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests/run_durable_storage_native.sh'

class TestS3Fix01_1(unittest.TestCase):
    def test_durable_storage_native_tests(self):
        self.assertTrue(SCRIPT.exists(), f'Missing run script: {SCRIPT}')
        r = subprocess.run(
            ['bash', str(SCRIPT)],
            capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        # Require all three modes to pass
        for mode in ('storage-error-stops-io', 'unp-seven-digits', 'restart-no-unp-reuse'):
            self.assertIn(f'{mode} PASS', r.stdout,
                f'Expected "{mode} PASS" in output:\n{r.stdout}')

if __name__ == '__main__':
    unittest.main()
