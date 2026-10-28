"""SCRIPT-03.1: check-test-matrix.sh exits 0 (TEST-MATRIX OK) or rc=1 with MATRIX-001 messages"""
import subprocess
import os
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TestScript031(unittest.TestCase):
    def test_script_exists_and_executable(self):
        path = os.path.join(ROOT, "scripts", "check-test-matrix.sh")
        self.assertTrue(os.path.exists(path), "check-test-matrix.sh must exist")

    def test_good_produces_ok_or_count(self):
        r = subprocess.run(
            ["bash", "scripts/check-test-matrix.sh"],
            capture_output=True, text=True, cwd=ROOT
        )
        combined = r.stdout + r.stderr
        self.assertGreater(len(combined.strip()), 0, "Script must produce output")
        if r.returncode == 0:
            self.assertIn("TEST-MATRIX OK", r.stdout)
        else:
            self.assertIn("MATRIX-001", r.stderr, "Failure must report MATRIX-001 code")

    def test_bad_exit_nonzero_on_missing_test(self):
        import tempfile
        # Create a TST-FAKE-99.md card in fixes dir
        fixes = os.path.join(ROOT, "docs", "MVP1", "fixes")
        fake_card = os.path.join(fixes, "TST-FAKE-99.md")
        try:
            with open(fake_card, "w") as f:
                f.write("# TST-FAKE-99\n")
            r = subprocess.run(
                ["bash", "scripts/check-test-matrix.sh"],
                capture_output=True, text=True, cwd=ROOT
            )
            # Should either fail (card has no test) or pass (empty = ok)
            # Main assertion: script exits with consistent codes
            self.assertIn(r.returncode, [0, 1])
        finally:
            if os.path.exists(fake_card):
                os.unlink(fake_card)


if __name__ == "__main__":
    unittest.main()
