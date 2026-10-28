"""VG-16.2: UUID/TLV BLE not guessed — spec_pending:true for unconfirmed constants.

DOC-V3 is obtained (2026-10-23) but per DATECSPAY_PROTOCOL.md B-2:
  "BLE UUID профиля в PDF отсутствуют (в коде взяты из Android SDK-примера вендора)"
Q-B1 remains open until Datecs confirms UUIDs in writing.
Therefore UUID constants in DatecsPayBleProfile.h remain spec_pending:true.
"""
import os
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TRACE = ROOT / 'docs/vendor/datecspay-constants-trace.md'
PROTOCOL_NOTES = ROOT / 'docs/vendor/DATECSPAY_PROTOCOL.md'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')


class TestVG16_2(unittest.TestCase):

    def _trace(self):
        self.assertTrue(TRACE.exists(), f'trace file not found: {TRACE}')
        return TRACE.read_text()

    def test_trace_file_exists(self):
        """datecspay-constants-trace.md must exist."""
        self.assertTrue(TRACE.exists(), f'trace file not found: {TRACE}')

    def test_at_least_one_spec_pending_true(self):
        """At least one constant must be marked spec_pending=true (UUIDs not in PDF)."""
        content = self._trace()
        pending_count = content.count('true')
        self.assertGreaterEqual(pending_count, 1,
                                'Must have at least one spec_pending=true entry in trace file')

    def test_service_uuid_prefix_is_spec_pending(self):
        """ServiceUuidPrefix must be spec_pending=true (UUID not confirmed in DOC-V3)."""
        content = self._trace()
        lines = content.splitlines()
        uuid_rows = [l for l in lines if 'ServiceUuidPrefix' in l]
        self.assertGreater(len(uuid_rows), 0,
                           'ServiceUuidPrefix must appear in the trace table')
        row = uuid_rows[0]
        self.assertIn('true', row,
                      'ServiceUuidPrefix row must contain "true" for spec_pending '
                      '(UUID not in DOC-V3 per B-2)')

    def test_all_uuid_constants_are_spec_pending(self):
        """All 5 UUID constants must be spec_pending=true."""
        content = self._trace()
        uuid_constants = [
            'ServiceUuidPrefix',
            'PowerCharacteristic',
            'WakeCharacteristic',
            'ReadCharacteristic',
            'WriteCharacteristic',
        ]
        for name in uuid_constants:
            rows = [l for l in content.splitlines() if name in l]
            self.assertGreater(len(rows), 0, f'{name} must appear in trace')
            self.assertIn('true', rows[0],
                          f'{name} must be spec_pending=true (UUID not in PDF)')

    def test_vendor_chunk_is_spec_pending(self):
        """VendorWriteChunk must be spec_pending=true (chunk size not confirmed, Q-B1)."""
        content = self._trace()
        rows = [l for l in content.splitlines() if 'VendorWriteChunk' in l]
        self.assertGreater(len(rows), 0, 'VendorWriteChunk must appear in trace')
        self.assertIn('true', rows[0],
                      'VendorWriteChunk must be spec_pending=true (Q-B1 pending)')

    def test_protocol_notes_reference_b2_issue(self):
        """DATECSPAY_PROTOCOL.md must document B-2 (UUIDs not in PDF) as evidence."""
        self.assertTrue(PROTOCOL_NOTES.exists(),
                        f'DATECSPAY_PROTOCOL.md not found: {PROTOCOL_NOTES}')
        content = PROTOCOL_NOTES.read_text()
        self.assertIn('B-2', content,
                      'DATECSPAY_PROTOCOL.md must document B-2 (UUID not in PDF)')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_16_2.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
