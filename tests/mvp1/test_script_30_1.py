"""SCRIPT-30.1: scripts/hil/run.sh — exits 3 without HIL_DEVICE_HOST or HIL_CONFIRM"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript301(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "hil", "run.sh")))

    def test_exits_3_without_confirm(self):
        env = os.environ.copy()
        env.pop("HIL_DEVICE_HOST", None)
        env.pop("HIL_CONFIRM", None)
        r = subprocess.run(
            ["bash", "scripts/hil/run.sh", "--device", "bc50", "--scenario", "HIL-BC-01", "--out", "/tmp/hil-test-out"],
            capture_output=True, text=True, cwd=ROOT, env=env
        )
        self.assertIn(r.returncode, [1, 3], f"Must exit 1 or 3 without HIL env\n{r.stdout}\n{r.stderr}")
        self.assertRegex(r.stderr, r'HIL-ENV', "Must print HIL-ENV message")

    def test_missing_args_usage(self):
        r = subprocess.run(
            ["bash", "scripts/hil/run.sh"],
            capture_output=True, text=True, cwd=ROOT
        )
        self.assertNotEqual(r.returncode, 0, "Must fail without required args")

if __name__ == "__main__": unittest.main()
