"""TG-015 — scripts/check-migrations.sh exists and exits 0 (or 3 for missing infra)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG015MigrationsCheck(unittest.TestCase):
    def test_check_migrations_script_exists(self):
        script = ROOT / "scripts" / "check-migrations.sh"
        self.assertTrue(script.exists(), f"check-migrations.sh not found at {script}")

    def test_check_migrations_runs(self):
        script = ROOT / "scripts" / "check-migrations.sh"
        result = subprocess.run(
            ["bash", str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        # Exit 0 = migrations valid; exit 3 = no migrations dir (also acceptable)
        self.assertIn(
            result.returncode, (0, 3),
            f"check-migrations.sh unexpected rc={result.returncode}\n"
            + result.stdout[-2000:] + "\n" + result.stderr[-1000:],
        )


if __name__ == "__main__":
    unittest.main()
