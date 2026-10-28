"""VG-20.2 — Golden vectors coverage: Kotlin host test covers cmd48×2, cmd49, cmd56, cmd60, cmd62, cmd74, cmd76, cmd103."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
RUNNER = ROOT / "SmartDevices" / "bluecash-app" / "tests" / "host" / "run-datecs-codec.py"
HOST_KT = ROOT / "SmartDevices" / "bluecash-app" / "tests" / "host" / "DatecsCodecVendorHost.kt"


REQUIRED_GOLDEN_CMDS = [
    "golden cmd56_close",
    "golden cmd60_cancel",
    "golden cmd76_txn_status",
    "golden cmd103_receipt_info",
    "golden cmd62_read_datetime",
    "golden cmd74_status",
    "golden cmd48_open_syntax1",
    "golden cmd48_open_syntax2",
    "golden cmd49_sale_cp1251",
    "golden cmd53_pay_cash BCC independent",
    "golden cmd56_answer decode",
    "golden cmd76_answer decode",
]


class TestVG202GoldenVectorCoverage(unittest.TestCase):
    def test_kotlin_source_has_all_required_golden_tests(self):
        self.assertTrue(HOST_KT.exists(), f"DatecsCodecVendorHost.kt not found at {HOST_KT}")
        src = HOST_KT.read_text()
        for label in REQUIRED_GOLDEN_CMDS:
            self.assertIn(
                label, src,
                f"DatecsCodecVendorHost.kt must contain golden test '{label}'"
            )

    def test_kotlin_host_passes_with_all_golden_vectors(self):
        result = subprocess.run(
            ["python3", str(RUNNER)],
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            f"Kotlin host test failed.\nstdout: {result.stdout}\nstderr: {result.stderr}"
        )


if __name__ == "__main__":
    unittest.main()
