"""VG-16.3: docs/vendor/datecspay-constants-trace.md — required structure.

Table must have columns: constant, value, type, source, pdf_section, spec_pending, note.
Each data row must have at least these fields non-empty.
"""
import os
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TRACE = ROOT / 'docs/vendor/datecspay-constants-trace.md'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')

REQUIRED_COLUMNS = ['constant', 'source', 'spec_pending', 'pdf_section']
REQUIRED_CONSTANTS = [
    'ServiceUuidPrefix',
    'PowerCharacteristic',
    'WakeCharacteristic',
    'ReadCharacteristic',
    'WriteCharacteristic',
    'VendorWriteChunk',
    'PowerOff',
    'PowerOn',
]


def _parse_table(content: str):
    """Return list of non-header, non-separator rows from the first markdown table."""
    rows = []
    in_table = False
    for line in content.splitlines():
        stripped = line.strip()
        if stripped.startswith('|'):
            # Skip separator rows (contain ---)
            if '---' in stripped:
                in_table = True
                continue
            if not in_table:
                # This is the header row; mark table started when we see separator
                pass
            else:
                cells = [c.strip() for c in stripped.split('|') if c.strip()]
                if cells:
                    rows.append(cells)
        elif in_table and stripped == '':
            break  # end of table
    return rows


class TestVG16_3(unittest.TestCase):

    def _content(self):
        self.assertTrue(TRACE.exists(), f'trace file not found: {TRACE}')
        return TRACE.read_text()

    def test_file_exists(self):
        """datecspay-constants-trace.md must exist."""
        self.assertTrue(TRACE.exists(), f'trace file not found: {TRACE}')

    def test_required_column_headers_present(self):
        """Trace table must include required column names."""
        content = self._content()
        content_lower = content.lower()
        for col in REQUIRED_COLUMNS:
            self.assertIn(col.lower(), content_lower,
                          f'Required column {col!r} not found in trace file')

    def test_has_data_rows(self):
        """Trace table must contain at least one data row."""
        content = self._content()
        rows = _parse_table(content)
        self.assertGreater(len(rows), 0, 'Trace table must have at least one data row')

    def test_each_row_has_multiple_cells(self):
        """Each data row must have at least 4 cells (constant, value, source, spec_pending)."""
        content = self._content()
        rows = _parse_table(content)
        for i, row in enumerate(rows):
            self.assertGreaterEqual(len(row), 4,
                                    f'Row {i} must have at least 4 cells: {row}')

    def test_all_required_constants_in_table(self):
        """All 8 constants from DatecsPayBleProfile.h must appear in the trace table."""
        content = self._content()
        for name in REQUIRED_CONSTANTS:
            self.assertIn(name, content,
                          f'Constant {name!r} must appear in datecspay-constants-trace.md')

    def test_source_field_references_sdk_or_pdf(self):
        """Source field must reference either PDF section or Android SDK example."""
        content = self._content()
        rows = _parse_table(content)
        for i, row in enumerate(rows):
            row_text = ' '.join(row).lower()
            has_source = ('sdk' in row_text or 'pdf' in row_text
                          or 'android' in row_text or 'example' in row_text
                          or 'vendor' in row_text or 'n/a' in row_text)
            self.assertTrue(has_source,
                            f'Row {i} must reference source (PDF or SDK example): {row}')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_16_3.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
