"""DSY-02.1 — DaisyCodec::decode golden answer frame cmd 48 (UNP DY000694-OP01-0000018)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY021(unittest.TestCase):
    def test_golden_daisy_cmd48(self):
        result = subprocess.run(
            ['bash', 'IoT/protocol-abstraction/run-tests.sh'],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            f'run-tests.sh failed\nstdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}',
        )
        self.assertIn(
            'golden_daisy_cmd48=pass', result.stdout,
            f'marker not in stdout: {result.stdout[-2000:]}',
        )
        self.assertIn('runtime vendor codec tests: OK', result.stdout)


if __name__ == '__main__':
    unittest.main()
