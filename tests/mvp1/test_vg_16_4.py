"""VG-16.4: Row count in datecspay-constants-trace.md = constexpr count in DatecsPayBleProfile.h.

Acceptance: scripts/check-vendor-docs.sh + datecspay-constants-trace.md:
row count == constant count in DatecsPayBleProfile.h.
"""
import os
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TRACE = ROOT / 'docs/vendor/datecspay-constants-trace.md'
HEADER = ROOT / 'IoT/common-modules/datecspay/DatecsPayBleProfile.h'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')


def _count_constants(header_text: str) -> int:
    """Count static constexpr declarations in the header."""
    return len(re.findall(r'\bstatic\s+constexpr\b', header_text))


def _count_trace_rows(trace_text: str) -> int:
    """Count data rows (non-header, non-separator) in the markdown table."""
    rows = []
    in_table = False
    for line in trace_text.splitlines():
        stripped = line.strip()
        if stripped.startswith('|'):
            if '---' in stripped:
                in_table = True
                continue
            if not in_table:
                continue
            cells = [c.strip() for c in stripped.split('|') if c.strip()]
            if cells:
                rows.append(cells)
        elif in_table and stripped == '':
            break
    return len(rows)


class TestVG16_4(unittest.TestCase):

    def test_header_has_constants(self):
        """DatecsPayBleProfile.h must declare at least one constexpr constant."""
        self.assertTrue(HEADER.exists(), f'Header not found: {HEADER}')
        count = _count_constants(HEADER.read_text())
        self.assertGreater(count, 0, 'Header must have at least one static constexpr')

    def test_trace_file_exists(self):
        """datecspay-constants-trace.md must exist."""
        self.assertTrue(TRACE.exists(), f'Trace file not found: {TRACE}')

    def test_trace_row_count_equals_constant_count(self):
        """Row count in trace must equal constexpr constant count in header."""
        self.assertTrue(HEADER.exists(), f'Header not found: {HEADER}')
        self.assertTrue(TRACE.exists(), f'Trace file not found: {TRACE}')

        constant_count = _count_constants(HEADER.read_text())
        self.assertGreater(constant_count, 0, 'Must have at least one constant in header')

        trace_row_count = _count_trace_rows(TRACE.read_text())
        self.assertEqual(
            trace_row_count, constant_count,
            f'Trace has {trace_row_count} rows but header has {constant_count} constants; '
            f'each constant must have exactly one trace entry'
        )

    def test_trace_has_eight_rows(self):
        """Trace must have exactly 8 rows (matching the 8 constants)."""
        self.assertTrue(TRACE.exists(), f'Trace file not found: {TRACE}')
        trace_row_count = _count_trace_rows(TRACE.read_text())
        self.assertEqual(trace_row_count, 8,
                         f'Expected 8 trace rows (one per constant), got {trace_row_count}')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_16_4.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
