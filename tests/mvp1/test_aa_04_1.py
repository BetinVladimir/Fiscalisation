"""AA-04.1 — AdminApp Jest setup: npx jest --version exits 0."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
ADMINAPP = ROOT / 'AdminApp'


class TestAA04_1(unittest.TestCase):
    def test_jest_installed_and_runnable(self):
        r = subprocess.run(
            ['npx', '--no-install', 'jest', '--version'],
            cwd=str(ADMINAPP),
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(
            r.returncode, 0,
            f'npx jest --version failed (rc={r.returncode}):\n{r.stdout}\n{r.stderr}',
        )
        combined = r.stdout + r.stderr
        # Jest prints its version, e.g. "29.7.0"
        self.assertRegex(
            combined, r'\d+\.\d+\.\d+',
            f'Expected jest version number in output:\n{combined}',
        )


if __name__ == '__main__':
    unittest.main()
