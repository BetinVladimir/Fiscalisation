"""VG-20.3 — Decode cmd56_answer and cmd76_answer: BCC and STAT verified in Kotlin host."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
RUNNER = ROOT / "SmartDevices" / "bluecash-app" / "tests" / "host" / "run-datecs-codec.py"
HOST_KT = ROOT / "SmartDevices" / "bluecash-app" / "tests" / "host" / "DatecsCodecVendorHost.kt"


class TestVG203DecodeAnswerVectors(unittest.TestCase):
    def test_kotlin_source_has_answer_decode_tests(self):
        self.assertTrue(HOST_KT.exists(), f"DatecsCodecVendorHost.kt not found at {HOST_KT}")
        src = HOST_KT.read_text()
        self.assertIn("golden cmd56_answer decode", src,
                      "DatecsCodecVendorHost.kt must contain 'golden cmd56_answer decode' test")
        self.assertIn("golden cmd76_answer decode", src,
                      "DatecsCodecVendorHost.kt must contain 'golden cmd76_answer decode' test")

    def test_cmd56_answer_bcc_independent(self):
        """Verify cmd56_answer BCC is correct using independent Python computation."""
        raw = bytes.fromhex(
            "01 30 30 33 39 29 30 30 33 38 30 09 32 36 39 09"
            " 04 80 80 80 80 80 82 80 80 05 30 36 3A 3E 03".replace(" ", "")
        )
        # BCC = sum of bytes[1..checksumAt] where checksumAt is position of ENQ (0x05)
        checksum_at = raw.index(0x05, 10)  # ENQ after data/STAT
        computed = sum(raw[1:checksum_at + 1]) & 0xFFFF
        stored_hex = raw[checksum_at + 1:checksum_at + 5]
        stored = int("".join(format(b - 0x30, 'x') for b in stored_hex), 16)
        self.assertEqual(computed, stored,
                         f"cmd56_answer BCC mismatch: computed={computed:#06x} stored={stored:#06x}")

    def test_cmd76_answer_bcc_independent(self):
        """Verify cmd76_answer BCC is correct using independent Python computation."""
        raw = bytes.fromhex(
            "01 30 30 34 37 32 30 30 34 3C 30 09 30 09 35 31"
            " 37 09 30 09 30 2E 30 30 09 30 2E 30 30 09 04 80"
            " 80 80 80 86 9A 80 80 05 30 38 3D 35 03".replace(" ", "")
        )
        checksum_at = raw.index(0x05, 10)
        computed = sum(raw[1:checksum_at + 1]) & 0xFFFF
        stored_hex = raw[checksum_at + 1:checksum_at + 5]
        stored = int("".join(format(b - 0x30, 'x') for b in stored_hex), 16)
        self.assertEqual(computed, stored,
                         f"cmd76_answer BCC mismatch: computed={computed:#06x} stored={stored:#06x}")

    def test_kotlin_host_passes(self):
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
