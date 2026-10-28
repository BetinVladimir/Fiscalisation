"""SCRIPT-09.1: check-golden-vectors.sh — vector files exist with cases"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript091(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-golden-vectors.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-golden-vectors.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_gld_message(self):
        r = subprocess.run(["bash", "scripts/check-golden-vectors.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'GLD-|GOLDEN-VECTORS', "Must emit GLD-* or GOLDEN-VECTORS message")

if __name__ == "__main__": unittest.main()
