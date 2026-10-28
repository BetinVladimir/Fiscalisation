"""TG-016 — scripts/loadtest/run.sh exists and --dry-run prints duplicates=0 lost=0."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG016LoadTest(unittest.TestCase):
    def test_loadtest_script_exists(self):
        script = ROOT / "scripts" / "loadtest" / "run.sh"
        self.assertTrue(script.exists(), f"scripts/loadtest/run.sh not found at {script}")

    def test_loadtest_dry_run_prints_expected_output(self):
        script = ROOT / "scripts" / "loadtest" / "run.sh"
        result = subprocess.run(
            ["bash", str(script), "--dry-run"],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(
            result.returncode, 0,
            f"loadtest --dry-run exited {result.returncode}\n"
            + result.stdout + "\n" + result.stderr,
        )
        output = result.stdout + result.stderr
        self.assertIn("duplicates=0", output,
                      "Expected 'duplicates=0' in dry-run output")
        self.assertIn("lost=0", output,
                      "Expected 'lost=0' in dry-run output")


if __name__ == "__main__":
    unittest.main()
