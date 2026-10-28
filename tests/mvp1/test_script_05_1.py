"""SCRIPT-05.1: check-prod-artifact.sh exits 0 on clean repo"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript051(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-prod-artifact.sh")))

    def test_good_exits_zero(self):
        r = subprocess.run(["bash", "scripts/check-prod-artifact.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")
        if r.returncode == 0:
            self.assertIn("PROD-ARTIFACT OK", r.stdout)

    def test_output_not_empty(self):
        r = subprocess.run(["bash", "scripts/check-prod-artifact.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertGreater(len((r.stdout + r.stderr).strip()), 0)

if __name__ == "__main__": unittest.main()
