"""TST-DEEP-14.1 — DatecsPay golden frames: 72 frames parse/verify via C++ test.

Acceptance: IoT/common-modules/datecspay/tests/DatecsPayGoldenFramesTest.cpp exists;
bash run-golden-tests.sh compiles and runs it; stdout contains 'GOLDEN_FRAMES OK 72'.
"""
import os
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
DATECSPAY_DIR = ROOT / "IoT" / "common-modules" / "datecspay"
GOLDEN_TEST_CPP = DATECSPAY_DIR / "tests" / "DatecsPayGoldenFramesTest.cpp"
RUN_GOLDEN_SH = DATECSPAY_DIR / "run-golden-tests.sh"
FIXTURE = ROOT / "docs" / "MVP1" / "fixtures" / "datecspay_golden_frames.json"

EXPECTED_COUNT = 72


class TestTSTDEEP14_1(unittest.TestCase):
    def test_golden_frames_cpp_exists(self):
        self.assertTrue(
            GOLDEN_TEST_CPP.exists(),
            f"DatecsPayGoldenFramesTest.cpp not found: {GOLDEN_TEST_CPP}"
        )

    def test_run_golden_sh_exists(self):
        self.assertTrue(
            RUN_GOLDEN_SH.exists(),
            f"run-golden-tests.sh not found: {RUN_GOLDEN_SH}"
        )

    def test_fixture_has_72_frames(self):
        """Fixture must contain exactly 72 golden frames."""
        import json
        self.assertTrue(FIXTURE.exists(), f"fixture not found: {FIXTURE}")
        frames = json.loads(FIXTURE.read_text())
        self.assertEqual(
            len(frames), EXPECTED_COUNT,
            f"expected {EXPECTED_COUNT} frames, got {len(frames)}"
        )

    def test_fixture_all_xor_verified(self):
        """All 72 frames must have xor_verified=true in fixture."""
        import json
        self.assertTrue(FIXTURE.exists())
        frames = json.loads(FIXTURE.read_text())
        bad = [i for i, f in enumerate(frames) if not f.get("xor_verified")]
        self.assertEqual(
            bad, [],
            f"frames with xor_verified=false: {bad}"
        )

    def test_cpp_golden_frames_compile_and_pass(self):
        """C++ golden frames test compiles and passes 72/72."""
        self.assertTrue(RUN_GOLDEN_SH.exists())
        result = subprocess.run(
            ["bash", str(RUN_GOLDEN_SH)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=60,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"run-golden-tests.sh failed (rc={result.returncode}):\n{output}"
        )
        self.assertIn(
            f"GOLDEN_FRAMES OK {EXPECTED_COUNT}", output,
            f"expected 'GOLDEN_FRAMES OK {EXPECTED_COUNT}' in output:\n{output}"
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_deep_14_1.txt").write_text(output)


if __name__ == "__main__":
    unittest.main(verbosity=2)
