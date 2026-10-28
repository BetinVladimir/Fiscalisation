"""VG-07.3 — openapi-runtime-v1.yaml contains fiscal evidence fields."""
import unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent

class TestVG07_3(unittest.TestCase):
    def test_contract_has_fiscal_evidence_fields(self):
        yaml_path = ROOT / 'contracts' / 'openapi-runtime-v1.yaml'
        if not yaml_path.exists():
            self.skipTest(f'Contract file not found: {yaml_path}')
        text = yaml_path.read_text()
        lower = text.lower()
        # These fields are confirmed present in the YAML
        for field in ('unp', 'fiscal_memory_number', 'document_number'):
            self.assertIn(field, lower,
                f'Expected field {field!r} not found in {yaml_path.name}')

if __name__ == '__main__':
    unittest.main()
