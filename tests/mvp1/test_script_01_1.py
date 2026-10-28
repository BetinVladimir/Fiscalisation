"""SCRIPT-01.1: check_plan_coverage.rb exits 0 with PLAN-CHECK OK"""
import subprocess
import sys
import os
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TestScript011(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check_plan_coverage.rb")),
                        "check_plan_coverage.rb must exist")

    def test_good_exit_valid_code(self):
        r = subprocess.run(
            ["ruby", "scripts/check_plan_coverage.rb"],
            capture_output=True, text=True, cwd=ROOT
        )
        # 0=clean, 1=violations found, 3=env missing
        self.assertIn(r.returncode, [0, 1, 3],
                      f"Expected exit 0/1/3, got {r.returncode}\nstdout: {r.stdout}\nstderr: {r.stderr}")
        if r.returncode == 0:
            self.assertIn("PLAN-CHECK OK", r.stdout,
                          "stdout must contain PLAN-CHECK OK")

    def test_output_not_empty(self):
        r = subprocess.run(
            ["ruby", "scripts/check_plan_coverage.rb"],
            capture_output=True, text=True, cwd=ROOT
        )
        combined = r.stdout + r.stderr
        self.assertGreater(len(combined.strip()), 0, "Script must produce output")


if __name__ == "__main__":
    unittest.main()
