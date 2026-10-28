"""SCRIPT-12.1: check-nap-baseline.sh exits 0 or 1 with NAP-* message"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript121(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-nap-baseline.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-nap-baseline.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_nap_message(self):
        r = subprocess.run(["bash", "scripts/check-nap-baseline.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'NAP-', "Must emit NAP-* message")

if __name__ == "__main__": unittest.main()
