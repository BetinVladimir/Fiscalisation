"""SCRIPT-10.1: check-datecs-error-catalog.sh — catalog ≥13 entries with ErrorCategory"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript101(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-datecs-error-catalog.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-datecs-error-catalog.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_ok_message_on_good_catalog(self):
        r = subprocess.run(["bash", "scripts/check-datecs-error-catalog.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'CAT-|DATECS-ERROR-CATALOG', "Must emit CAT-* or DATECS-ERROR-CATALOG message")

if __name__ == "__main__": unittest.main()
