"""TST-S3-03.1 — Unsupported payment type NHIF rejected before reserve/I/O (BUG-042).

Verifies: command with payment type "NHIF" on Daisy profile fails decode
before reserve_command or executor call; no operation row created.
Uses real intent_processor.cpp + durable_storage.cpp compiled as a host binary.
"""
import os
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_intent


class TestTSTS303_1(unittest.TestCase):
    def test_unsupported_payment_nhif(self):
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
        self.assertIn('unsupported-payment-nhif', tests,
                      f'Test not found in TAP output:\n{stdout}')
        self.assertTrue(tests['unsupported-payment-nhif'],
                        f'unsupported-payment-nhif FAILED\n{stderr}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'tst-s3-03-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'tst_s3_03_1.tap').write_text(stdout)


if __name__ == '__main__':
    unittest.main()
