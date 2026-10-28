"""SCRIPT-13.1: check-norm-decisions.sh exits 0 or 1 with NRM-* or NORM-DECISIONS message"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript131(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-norm-decisions.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-norm-decisions.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_norm_message(self):
        r = subprocess.run(["bash", "scripts/check-norm-decisions.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'NRM-|NORM-DECISIONS', "Must emit NRM-* or NORM-DECISIONS message")

if __name__ == "__main__": unittest.main()
