"""TST-S3-01.1 — REJECTED_BEFORE_IO when executor.ready()==false (BUG-040).

Verifies: reserve_command not called, UNP counter unchanged, response carries
REJECTED_BEFORE_IO state, repeat after executor becomes ready uses same UNP.
Uses real intent_processor.cpp + durable_storage.cpp compiled as a host binary.
"""
import os
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_intent


class TestTSTS301_1(unittest.TestCase):
    def test_rejected_before_io(self):
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
        self.assertIn('rejected-before-io', tests,
                      f'Test not found in TAP output:\n{stdout}')
        self.assertTrue(tests['rejected-before-io'],
                        f'rejected-before-io FAILED\n{stderr}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'tst-s3-01-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'tst_s3_01_1.tap').write_text(stdout)


if __name__ == '__main__':
    unittest.main()
