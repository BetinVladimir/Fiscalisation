"""BUG-053.1 — FiscalStatusResponse has separate unp, fiscalNumber, fiscalReference fields."""
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG053_1(unittest.TestCase):
    def test_fiscal_status_response_has_separate_fields(self):
        api_client = POS / 'src/infrastructure/fiscal/FiscalApiClient.ts'
        if not api_client.exists():
            self.skipTest(f'{api_client} not found')
        content = api_client.read_text(encoding='utf-8')
        self.assertIn('unp?: string', content, 'FiscalStatusResponse must have unp field')
        self.assertIn('fiscalNumber?: number', content, 'FiscalStatusResponse must have fiscalNumber field')
        self.assertIn('fiscalReference?: string', content, 'FiscalStatusResponse must have fiscalReference field')

    def test_no_unp_fallback_to_fiscal_reference_in_type(self):
        """Type must not express unp as fallback for fiscalReference."""
        api_client = POS / 'src/infrastructure/fiscal/FiscalApiClient.ts'
        if not api_client.exists():
            self.skipTest(f'{api_client} not found')
        content = api_client.read_text(encoding='utf-8')
        self.assertNotIn('unp ?? fiscalReference', content)
        self.assertNotIn('unp ?? fiscal_reference', content)


if __name__ == '__main__':
    unittest.main()
