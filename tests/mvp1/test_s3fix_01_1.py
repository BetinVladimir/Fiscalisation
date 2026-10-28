"""S3FIX-01.1 — storage errors stop I/O, UNP saves 7 digits, restart no UNP reuse."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'IoT' / 'firmware' / 'edge-agent-s3' / 'idf' / 'tests' / 'run_durable_storage_native.sh'

REQUIRED_MARKERS = (
    'storage-error-stops-io PASS',
    'unp-seven-digits PASS',
    'restart-no-unp-reuse PASS',
)


class TestS3FIX01(unittest.TestCase):
    def test_durable_storage_invariants(self):
        self.assertTrue(SCRIPT.exists(), f'script not found: {SCRIPT}')
        result = subprocess.run(
            ['bash', str(SCRIPT)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'run_durable_storage_native.sh failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        for marker in REQUIRED_MARKERS:
            self.assertIn(
                marker, result.stdout,
                f'Required marker not found: {marker!r}\nOutput: {result.stdout[-2000:]}',
            )


if __name__ == '__main__':
    unittest.main()
