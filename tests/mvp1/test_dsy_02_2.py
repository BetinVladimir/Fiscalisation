"""DSY-02.2 — serial_exchange: SYN extends wait, NAK retry, 3000ms deadline."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY022(unittest.TestCase):
    def test_syn_nak_deadline_markers(self):
        result = subprocess.run(
            ['bash', 'IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh'],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            f'run_native_serial.sh failed\nstdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}',
        )
        for marker in ('syn_extends=pass', 'nak_retry=pass', 'deadline=pass'):
            self.assertIn(
                marker, result.stdout,
                f'marker {marker!r} not in stdout: {result.stdout[-2000:]}',
            )
        self.assertIn('actual serial runtime tests: OK', result.stdout)


if __name__ == '__main__':
    unittest.main()
