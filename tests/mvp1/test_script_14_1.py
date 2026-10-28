"""SCRIPT-14.1: check-decisions.sh exits 0 or 1; checks D-1..D-11"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript141(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-decisions.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-decisions.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_dec_message(self):
        r = subprocess.run(["bash", "scripts/check-decisions.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'DEC-|DECISIONS', "Must emit DEC-* or DECISIONS message")

if __name__ == "__main__": unittest.main()
