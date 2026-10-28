"""TG-009.4 — go vet ./internal/domain clean."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG009_4(unittest.TestCase):
    def test_go_vet_domain(self):
        result = subprocess.run(
            ['go', 'vet', './internal/domain/'],
            cwd=str(ROOT / 'fiscal-backend'),
            capture_output=True, text=True,
        )
        self.assertEqual(result.returncode, 0,
                         f'go vet failed:\n{result.stdout}\n{result.stderr}')


if __name__ == '__main__':
    unittest.main()
