"""BP-05.1 — BLE drop/UNKNOWN resolved via 0x1A+0x24; purchaseCalls==1."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
HERE = Path(__file__).parent
MAIN_INC = ROOT / "IoT" / "firmware" / "edge-agent-s3" / "idf" / "tests" / "bluepad_host" / "include"
SRC = HERE / "bp05_native_test.cpp"


class TestBP051(unittest.TestCase):
    def _compile_and_run(self, mode: str) -> str:
        self.assertTrue(SRC.exists(), f"test source not found: {SRC}")
        self.assertTrue(MAIN_INC.exists(), f"include dir not found: {MAIN_INC}")
        with tempfile.TemporaryDirectory(prefix="bp05-1-") as td:
            binary = str(Path(td) / "bp05")
            compile_result = subprocess.run(
                [
                    os.environ.get("CXX", "c++"),
                    "-std=c++17", "-Wall", "-Wextra", "-pthread",
                    f"-I{MAIN_INC}",
                    str(SRC),
                    "-o", binary,
                ],
                capture_output=True, text=True, timeout=60,
            )
            self.assertEqual(
                compile_result.returncode, 0,
                f"Compile failed:\n{compile_result.stderr}",
            )
            run_result = subprocess.run(
                [binary, mode],
                capture_output=True, text=True, timeout=30,
            )
            if run_result.returncode != 0:
                self.fail(
                    f"Test {mode!r} failed (rc={run_result.returncode}):\n"
                    f"stdout: {run_result.stdout}\nstderr: {run_result.stderr}"
                )
            return run_result.stdout

    def test_drop_event_resolves_to_committed(self):
        out = self._compile_and_run("bp-05-1-drop-resolve")
        self.assertIn("bp-05-1-drop-resolve PASS", out)


if __name__ == "__main__":
    unittest.main()
