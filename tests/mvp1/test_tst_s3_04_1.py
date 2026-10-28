"""TST-S3-04.1 — Payment total mismatch rejected before reserve/I/O (BUG-043).

Verifies: items Σ=1200¢, payments Σ=1199¢ → PAYMENT_SUM_MISMATCH (decode fails)
before reserve_command or executor call; no operation row created.
Uses real intent_processor.cpp + durable_storage.cpp compiled as a host binary.
"""
import os
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_intent


class TestTSTS304_1(unittest.TestCase):
    def test_payment_total_mismatch(self):
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
        self.assertIn('payment-total-mismatch', tests,
                      f'Test not found in TAP output:\n{stdout}')
        self.assertTrue(tests['payment-total-mismatch'],
                        f'payment-total-mismatch FAILED\n{stderr}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'tst-s3-04-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'tst_s3_04_1.tap').write_text(stdout)


if __name__ == '__main__':
    unittest.main()
