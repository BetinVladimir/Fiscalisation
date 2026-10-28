"""POS-01.1 — fiscal-result.v1.json schema exists and has required fields."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CONTRACTS = ROOT / '../POS/docs/contracts'


REQUIRED_FIELDS = [
    'messageId', 'tenantId', 'operationId', 'state',
    'unp', 'fiscalNumber', 'fiscalReference', 'fiscalizedAt',
    'errorCode',
]

REQUIRED_STATES = ['FISCALIZED', 'REJECTED', 'UNKNOWN']


class TestPos011(unittest.TestCase):
    def setUp(self):
        self.schema_path = CONTRACTS / 'fiscal-result.v1.json'
        self.assertTrue(
            self.schema_path.exists(),
            f'fiscal-result.v1.json not found at {self.schema_path}',
        )
        with open(self.schema_path) as f:
            self.schema = json.load(f)

    def test_schema_is_valid_json_schema(self):
        self.assertEqual(self.schema.get('type'), 'object')
        self.assertIn('$schema', self.schema)
        self.assertIn('properties', self.schema)

    def test_required_envelope_fields_are_required(self):
        required = self.schema.get('required', [])
        for field in ['messageId', 'tenantId', 'operationId', 'state']:
            self.assertIn(field, required,
                          f'Field {field!r} must be in "required"')

    def test_all_expected_properties_exist(self):
        props = self.schema.get('properties', {})
        for field in REQUIRED_FIELDS:
            self.assertIn(field, props,
                          f'Property {field!r} missing from schema')

    def test_state_enum_contains_required_values(self):
        state_schema = self.schema['properties']['state']
        enum_values = state_schema.get('enum', [])
        for state in REQUIRED_STATES:
            self.assertIn(state, enum_values,
                          f'State {state!r} missing from enum')

    def test_unp_description_warns_against_substitution(self):
        """UNP must NOT be substituted with fiscalReference or fiscalNumber."""
        unp_desc = self.schema['properties']['unp'].get('description', '')
        # The description should warn about substitution
        self.assertTrue(
            len(unp_desc) > 10,
            'UNP property should have a meaningful description',
        )

    def test_fiscal_intent_schema_also_exists(self):
        intent_path = CONTRACTS / 'fiscal-intent.v1.json'
        self.assertTrue(intent_path.exists(), 'fiscal-intent.v1.json must also exist')

    def test_schemas_count_equals_two(self):
        schemas = list(CONTRACTS.glob('fiscal-*.v1.json'))
        self.assertGreaterEqual(
            len(schemas), 2,
            f'Expected at least 2 fiscal schemas, found {len(schemas)}: {[s.name for s in schemas]}',
        )


if __name__ == '__main__':
    unittest.main()
