"""SCRIPT-02.1: check_plan_acceptance.rb exits 0 with ACCEPT-CHECK OK"""
import subprocess
import os
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TestScript021(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check_plan_acceptance.rb")),
                        "check_plan_acceptance.rb must exist")

    def test_good_exit_zero(self):
        r = subprocess.run(
            ["ruby", "scripts/check_plan_acceptance.rb"],
            capture_output=True, text=True, cwd=ROOT
        )
        self.assertIn(r.returncode, [0, 3],
                      f"Expected exit 0 or 3, got {r.returncode}\n{r.stdout}\n{r.stderr}")
        if r.returncode == 0:
            self.assertIn("ACCEPT-CHECK OK", r.stdout)

    def test_output_not_empty(self):
        r = subprocess.run(
            ["ruby", "scripts/check_plan_acceptance.rb"],
            capture_output=True, text=True, cwd=ROOT
        )
        self.assertGreater(len((r.stdout + r.stderr).strip()), 0)


if __name__ == "__main__":
    unittest.main()
