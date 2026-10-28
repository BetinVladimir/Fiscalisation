"""BUG-053.3 — LocalReceiptOperations uses fiscalReference, not unp, for fiscalRef."""
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG053_3(unittest.TestCase):
    def test_fiscal_ref_uses_fiscal_reference(self):
        ops = POS / 'src/application/workflows/LocalReceiptOperations.ts'
        if not ops.exists():
            self.skipTest(f'{ops} not found')
        content = ops.read_text(encoding='utf-8')
        self.assertIn('fiscalRef:result.fiscalReference', content,
            'fiscalRef must use result.fiscalReference')
        self.assertNotIn('fiscalRef:result.operationId', content,
            'fiscalRef must not use operationId')

    def test_empty_unp_guard_present(self):
        ops = POS / 'src/application/workflows/LocalReceiptOperations.ts'
        if not ops.exists():
            self.skipTest(f'{ops} not found')
        content = ops.read_text(encoding='utf-8')
        self.assertIn('if(!result.unp)return', content,
            'deliver() must guard against empty UNP')


if __name__ == '__main__':
    unittest.main()
