"""CODE-006.1: staticcheck U1000 reports zero unused symbols in fiscal-backend."""
import subprocess
import unittest
import shutil
import os


REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
FISCAL_BACKEND = os.path.join(REPO_ROOT, "fiscal-backend")
STATICCHECK = shutil.which("staticcheck") or os.path.expanduser("~/go/bin/staticcheck")


class TestStaticcheckU1000(unittest.TestCase):
    def test_staticcheck_installed(self):
        self.assertTrue(
            os.path.isfile(STATICCHECK),
            f"staticcheck not found at {STATICCHECK}; install with: go install honnef.co/go/tools/cmd/staticcheck@latest",
        )

    def test_no_unused_symbols(self):
        result = subprocess.run(
            [STATICCHECK, "-checks", "U1000", "./..."],
            cwd=FISCAL_BACKEND,
            capture_output=True,
            text=True,
        )
        self.assertEqual(
            result.returncode,
            0,
            f"staticcheck U1000 found unused symbols:\n{result.stdout}\n{result.stderr}",
        )
        self.assertEqual(
            result.stdout.strip(),
            "",
            f"Expected no output; got:\n{result.stdout}",
        )


if __name__ == "__main__":
    unittest.main()
