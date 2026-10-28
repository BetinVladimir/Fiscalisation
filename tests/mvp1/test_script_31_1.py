"""SCRIPT-31.1: scripts/loadtest/run.sh — exits 3 without env, dry-run exits 0"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript311(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "loadtest", "run.sh")))

    def test_thresholds_exist(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "loadtest", "thresholds.json")))

    def test_dry_run_exits_zero(self):
        r = subprocess.run(
            ["bash", "scripts/loadtest/run.sh", "--dry-run"],
            capture_output=True, text=True, cwd=ROOT
        )
        self.assertEqual(r.returncode, 0, f"Dry-run must exit 0\n{r.stdout}\n{r.stderr}")
        self.assertIn("LOADTEST DRY-RUN", r.stdout)

    def test_missing_args_exits_nonzero(self):
        env = os.environ.copy()
        env.pop("FISCAL_API_URL", None)
        r = subprocess.run(
            ["bash", "scripts/loadtest/run.sh"],
            capture_output=True, text=True, cwd=ROOT, env=env
        )
        self.assertNotEqual(r.returncode, 0, "Must fail without required args")

if __name__ == "__main__": unittest.main()
