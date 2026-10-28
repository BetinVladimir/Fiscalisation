"""BUG-053.5 — No fiscalRef: result.operationId in production source."""
import unittest
from pathlib import Path

POS_SRC = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app/src')


class TestBUG053_5(unittest.TestCase):
    def test_no_fiscal_ref_uses_operation_id(self):
        if not POS_SRC.exists():
            self.skipTest(f'POS src not found at {POS_SRC}')
        violations = []
        for ts_file in POS_SRC.rglob('*.ts'):
            parts = ts_file.parts
            if any(p in ('.store-build', 'node_modules') for p in parts):
                continue
            if ts_file.name.endswith('.test.ts'):
                continue
            try:
                content = ts_file.read_text(encoding='utf-8')
            except OSError:
                continue
            # Check for old pattern: fiscalRef assigned from operationId directly
            if 'fiscalRef: result.operationId' in content:
                violations.append(str(ts_file.relative_to(POS_SRC)))
        self.assertEqual(violations, [],
            'fiscalRef should use fiscalReference, not operationId:\n' + '\n'.join(violations))


if __name__ == '__main__':
    unittest.main()
