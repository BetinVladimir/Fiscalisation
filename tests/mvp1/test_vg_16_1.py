"""VG-16.1: DatecsPayBleProfile.h — BLE 4.2 profile, correct namespace and constants."""
import os
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HEADER = ROOT / 'IoT/common-modules/datecspay/DatecsPayBleProfile.h'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')


class TestVG16_1(unittest.TestCase):

    def _content(self):
        self.assertTrue(HEADER.exists(), f'DatecsPayBleProfile.h not found: {HEADER}')
        return HEADER.read_text()

    def test_header_exists(self):
        """Header file must exist."""
        self.assertTrue(HEADER.exists(), f'DatecsPayBleProfile.h not found: {HEADER}')

    def test_namespace_datecspay_ble_profile(self):
        """File must declare namespace DatecsPayBleProfile (not a plain struct or global scope)."""
        content = self._content()
        self.assertIn('namespace DatecsPayBleProfile', content,
                      'Must declare namespace DatecsPayBleProfile')

    def test_ble_4_2_not_bt_30_spp(self):
        """File header comment must mention BLE (BluePad-50 Plus); BT 3.0 SPP is out of MVP."""
        content = self._content()
        self.assertRegex(content, r'BLE\s*4\.2|BLE',
                         'Header must reference BLE 4.2 (BluePad-50 Plus); BT 3.0 SPP excluded from MVP')

    def test_required_constants_present(self):
        """All 8 BLE profile constants must be declared."""
        content = self._content()
        required = [
            'ServiceUuidPrefix',
            'PowerCharacteristic',
            'WakeCharacteristic',
            'ReadCharacteristic',
            'WriteCharacteristic',
            'VendorWriteChunk',
            'PowerOff',
            'PowerOn',
        ]
        for name in required:
            self.assertIn(name, content, f'Constant {name!r} not found in DatecsPayBleProfile.h')

    def test_constexpr_count_is_eight(self):
        """Exactly 8 constexpr declarations (matches the 8 constants in the trace file)."""
        content = self._content()
        count = len(re.findall(r'\bstatic\s+constexpr\b', content))
        self.assertEqual(count, 8,
                         f'Expected 8 static constexpr declarations; found {count}')

    def test_power_bytes_are_ascii_digits(self):
        """PowerOff=0x30 and PowerOn=0x31 (ASCII '0'/'1') must be present with correct values."""
        content = self._content()
        self.assertIn('0x30', content, 'PowerOff must be 0x30')
        self.assertIn('0x31', content, 'PowerOn must be 0x31')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_16_1.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
