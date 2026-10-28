"""SCRIPT-26.1: check-version-bump.sh exits 0 or 1 with VER-* message"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript261(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-version-bump.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-version-bump.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_ver_message(self):
        r = subprocess.run(["bash", "scripts/check-version-bump.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'VER-|VERSION-BUMP', "Must emit VER-* or VERSION-BUMP message")

if __name__ == "__main__": unittest.main()
