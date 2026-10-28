"""TST-S3-05.1 — PAYLOAD_CONFLICT delivered when same op_id has different payload (BUG-044).

Verifies: second command with same operation_id but different canonical payload
delivers PAYLOAD_CONFLICT to result_sink (was previously silent); original
operation is not modified; executor not called a second time.
Uses real intent_processor.cpp + durable_storage.cpp compiled as a host binary.
"""
import os
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_intent


class TestTSTS305_1(unittest.TestCase):
    def test_payload_conflict_result_delivered(self):
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
        self.assertIn('payload-conflict-result-delivered', tests,
                      f'Test not found in TAP output:\n{stdout}')
        self.assertTrue(tests['payload-conflict-result-delivered'],
                        f'payload-conflict-result-delivered FAILED\n{stderr}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'tst-s3-05-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'tst_s3_05_1.tap').write_text(stdout)


if __name__ == '__main__':
    unittest.main()
