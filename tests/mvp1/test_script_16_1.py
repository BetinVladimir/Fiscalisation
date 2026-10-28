"""SCRIPT-16.1: check-doc-drift.sh exits 0 or 1 with DRF-* message"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript161(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-doc-drift.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-doc-drift.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_doc_drift_message(self):
        r = subprocess.run(["bash", "scripts/check-doc-drift.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'DRF-|DOC-DRIFT|DRIFT-', "Must emit DRF-* or DOC-DRIFT or DRIFT-* message")

if __name__ == "__main__": unittest.main()
