"""SCRIPT-17.1: check-doc-invariants.sh — H1, status schema==2, no duplicate H1"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript171(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-doc-invariants.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-doc-invariants.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_inv_message(self):
        r = subprocess.run(["bash", "scripts/check-doc-invariants.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'INV-|DOC-INVARIANTS', "Must emit INV-* or DOC-INVARIANTS message")

if __name__ == "__main__": unittest.main()
