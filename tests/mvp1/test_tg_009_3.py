"""TG-009.3 — scripts/check-storno-coverage.sh passes."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG009_3(unittest.TestCase):
    def test_storno_coverage_script(self):
        script = ROOT / 'scripts' / 'check-storno-coverage.sh'
        self.assertTrue(script.exists(), f'check-storno-coverage.sh not found')
        result = subprocess.run(
            ['bash', str(script)],
            capture_output=True, text=True,
        )
        self.assertEqual(result.returncode, 0,
                         f'coverage script failed:\n{result.stdout}\n{result.stderr}')
        self.assertIn('storno_coverage=pass', result.stdout)


if __name__ == '__main__':
    unittest.main()
