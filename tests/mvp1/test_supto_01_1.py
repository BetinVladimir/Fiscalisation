"""SUPTO-01.1 — supto_profiles.json fixture has 4 valid profiles."""
import json, unittest
from pathlib import Path

FIXTURE = Path(__file__).resolve().parents[2] / 'docs/MVP1/fixtures/supto_profiles.json'
REQUIRED_EVENTS = {'storno', 'cancel'}
REQUIRED_IDS = {'BC50', 'DATECS_S3', 'DAISY_USB', 'SMART_S'}


class TestSupto01_1(unittest.TestCase):
    def setUp(self):
        with open(FIXTURE) as f:
            self.data = json.load(f)
        self.profiles = {p['id']: p for p in self.data['profiles']}

    def test_has_four_profiles(self):
        self.assertEqual(set(self.profiles.keys()), REQUIRED_IDS)

    def test_all_have_required_audit_events(self):
        for pid, p in self.profiles.items():
            missing = REQUIRED_EVENTS - set(p['audit_events'])
            self.assertFalse(missing, f'{pid}: missing audit_events {missing}')

    def test_all_have_unsupported_with_reason(self):
        for pid, p in self.profiles.items():
            self.assertTrue(p['unsupported'], f'{pid}: unsupported must not be empty')
            self.assertTrue(p['unsupported_reason'], f'{pid}: unsupported_reason must be set')

    def test_bc50_disabled_operator(self):
        self.assertTrue(self.profiles['BC50']['disabled_operator'])

    def test_all_read_only_auditor(self):
        for pid, p in self.profiles.items():
            self.assertTrue(p['read_only_auditor'], f'{pid}: read_only_auditor must be true')


if __name__ == '__main__':
    unittest.main()
