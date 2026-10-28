"""TST-VENDOR-02.1 — C++ DatecsProtocolTest + RuntimeCodecVendorTest vendor codec vectors."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
IOT = ROOT / "IoT"
COMMON_DATECS = IOT / "common-modules/datecs"
PROTOCOL_ABS = IOT / "protocol-abstraction"

DATECS_MARKER = "Datecs protocol tests passed"
VENDOR_MARKER = "runtime vendor codec tests: OK"


class TestTSTVendor02_1(unittest.TestCase):
    def test_datecs_common_module_protocol_tests(self):
        script = COMMON_DATECS / "run-tests.sh"
        self.assertTrue(script.exists(), f"run-tests.sh not found: {script}")
        result = subprocess.run(
            ["bash", str(script)],
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            f"datecs run-tests.sh rc={result.returncode}\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}",
        )
        self.assertIn(
            DATECS_MARKER, result.stdout + result.stderr,
            f"Marker not found: {result.stdout[-3000:]}",
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tst-vendor-02-1-datecs"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)

    def test_runtime_codec_vendor_test(self):
        test_src = PROTOCOL_ABS / "tests/RuntimeCodecVendorTest.cpp"
        self.assertTrue(test_src.exists(), f"RuntimeCodecVendorTest.cpp not found: {test_src}")
        with tempfile.NamedTemporaryFile(prefix="beefiscal-vendor-02-", delete=False) as f:
            out_bin = f.name
        try:
            compile_result = subprocess.run(
                [
                    "c++", "-std=c++17", "-Wall", "-Wextra", "-Werror",
                    f"-I{PROTOCOL_ABS / 'tests'}",
                    f"-I{PROTOCOL_ABS / 'include'}",
                    f"-I{PROTOCOL_ABS / 'src'}",
                    str(PROTOCOL_ABS / "src/FrameCodec.cpp"),
                    str(test_src),
                    "-o", out_bin,
                ],
                capture_output=True,
                text=True,
                timeout=60,
            )
            self.assertEqual(
                compile_result.returncode, 0,
                f"Compile failed:\n{compile_result.stderr[-3000:]}",
            )
            run_result = subprocess.run(
                [out_bin],
                capture_output=True,
                text=True,
                timeout=30,
            )
            self.assertEqual(
                run_result.returncode, 0,
                f"RuntimeCodecVendorTest rc={run_result.returncode}\n"
                f"stdout: {run_result.stdout[-3000:]}\nstderr: {run_result.stderr[-2000:]}",
            )
            self.assertIn(
                VENDOR_MARKER, run_result.stdout,
                f"Marker not found: {run_result.stdout[-3000:]}",
            )
            artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
            if artifact_dir:
                out = Path(artifact_dir) / "tst-vendor-02-1-runtime"
                out.mkdir(parents=True, exist_ok=True)
                (out / "stdout.log").write_text(run_result.stdout)
        finally:
            try:
                os.unlink(out_bin)
            except OSError:
                pass


if __name__ == "__main__":
    unittest.main()
