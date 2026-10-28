"""TG-003.3 — device_command_result.json has device_time (RFC3339) and drift_seconds fields."""
import json, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


class TestTG003_3(unittest.TestCase):
    def test_device_time_contract_fields(self):
        f = ROOT / 'fiscal-backend' / 'contracts' / 'device_command_result.json'
        self.assertTrue(f.exists(), f'Contract file not found: {f}')
        data = json.loads(f.read_text())
        self.assertIn('DEVICE_TIME', data, 'Missing DEVICE_TIME key in contract')
        dt = data['DEVICE_TIME']
        self.assertIn('fields', dt, 'DEVICE_TIME missing "fields"')
        fields = dt['fields']
        self.assertIn('device_time', fields, 'Missing field: device_time')
        self.assertIn('drift_seconds', fields, 'Missing field: drift_seconds')
        self.assertIn('operation_id', fields, 'Missing field: operation_id')
        self.assertEqual(fields['device_time']['format'], 'RFC3339',
            'device_time must have format=RFC3339')
        self.assertEqual(fields['drift_seconds']['type'], 'integer',
            'drift_seconds must be integer type')


if __name__ == '__main__':
    unittest.main()
