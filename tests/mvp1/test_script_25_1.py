"""SCRIPT-25.1: check-data-location.sh exits 0 or 1 with DLC-* message"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript251(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-data-location.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-data-location.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_dlc_message(self):
        r = subprocess.run(["bash", "scripts/check-data-location.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'DLC-|DATA-LOCATION', "Must emit DLC-* or DATA-LOCATION message")

if __name__ == "__main__": unittest.main()
