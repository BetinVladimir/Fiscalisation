"""SCRIPT-08.1: check-vendor-docs.sh exits 0 or 1 (VND-* codes)"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript081(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-vendor-docs.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-vendor-docs.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_vnd_message(self):
        r = subprocess.run(["bash", "scripts/check-vendor-docs.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'VND-|VENDOR-DOCS', "Must emit VND-* or VENDOR-DOCS message")

if __name__ == "__main__": unittest.main()
