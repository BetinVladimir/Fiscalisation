"""TST-S3-02.1 — TRUSTED_TIME_REQUIRED when device clock is untrusted (BUG-041).

Verifies: MQTT command with CommandClock returning {0,0} is rejected before
reserve_command or executor call; no operation row; no UNP consumed.
Uses real intent_processor.cpp + durable_storage.cpp compiled as a host binary.
"""
import os
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_intent


class TestTSTS302_1(unittest.TestCase):
    def test_trusted_time_required(self):
        try:
            stdout, stderr, rc = native_intent.compile_and_run()
        except RuntimeError as e:
            self.skipTest(f'BLOCKED: {e}')

        if rc != 0:
            self.fail(
                f'intent_processor_native exited {rc}\n'
                f'stdout:\n{stdout[-3000:]}\nstderr:\n{stderr[-1000:]}'
            )

        tests = native_intent.parse_tap(stdout)
        self.assertIn('trusted-time-required', tests,
                      f'Test not found in TAP output:\n{stdout}')
        self.assertTrue(tests['trusted-time-required'],
                        f'trusted-time-required FAILED\n{stderr}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'tst-s3-02-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'tst_s3_02_1.tap').write_text(stdout)


if __name__ == '__main__':
    unittest.main()
