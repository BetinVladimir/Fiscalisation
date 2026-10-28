"""BUG-032.5 — Operator password stored in Keystore wrapper (SecretStore), not plaintext."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SECRET_STORE = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/SecretStore.kt'
MAIN_ACT = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt'


class TestBUG0325(unittest.TestCase):
    def setUp(self):
        self.assertTrue(SECRET_STORE.exists(), f'SecretStore.kt not found: {SECRET_STORE}')
        self.ss_src = SECRET_STORE.read_text()

    def test_secret_store_class_exists(self):
        self.assertRegex(
            self.ss_src,
            r'class\s+SecretStore\b',
            'SecretStore class must exist',
        )

    def test_operator_password_stored_encrypted(self):
        self.assertIn(
            'DeviceCredentialStore',
            self.ss_src,
            'SecretStore must delegate to DeviceCredentialStore (Keystore AES/GCM)',
        )

    def test_operator_password_put_method(self):
        self.assertRegex(
            self.ss_src,
            r'fun\s+putOperatorPassword\s*\(',
            'SecretStore must expose putOperatorPassword()',
        )

    def test_operator_password_get_method(self):
        self.assertRegex(
            self.ss_src,
            r'fun\s+getOperatorPassword\s*\(',
            'SecretStore must expose getOperatorPassword()',
        )

    def test_secret_not_stored_plain(self):
        for klass in ('SharedPreferences', 'putString("operator_password"'):
            self.assertNotIn(
                klass,
                self.ss_src,
                f'SecretStore must not store password in plaintext SharedPreferences ({klass!r})',
            )


if __name__ == '__main__':
    unittest.main()
