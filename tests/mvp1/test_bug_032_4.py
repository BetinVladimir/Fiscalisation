"""BUG-032.4 — BootReceiver starts service on BOOT_COMPLETED if device is activated."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BOOT_RECV = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BootReceiver.kt'


class TestBUG0324(unittest.TestCase):
    def setUp(self):
        self.assertTrue(BOOT_RECV.exists(), f'BootReceiver.kt not found: {BOOT_RECV}')
        self.src = BOOT_RECV.read_text()

    def test_class_extends_broadcast_receiver(self):
        self.assertRegex(
            self.src,
            r'class\s+BootReceiver\s*:\s*BroadcastReceiver\b',
            'BootReceiver must extend BroadcastReceiver',
        )

    def test_on_receive_declared(self):
        self.assertIn(
            'override fun onReceive(',
            self.src,
            'onReceive must be overridden',
        )

    def test_checks_boot_completed_action(self):
        self.assertIn(
            'BOOT_COMPLETED',
            self.src,
            'onReceive must check ACTION_BOOT_COMPLETED',
        )

    def test_starts_foreground_service(self):
        self.assertIn(
            'startForegroundService(',
            self.src,
            'BootReceiver must call startForegroundService()',
        )

    def test_checks_activation_before_starting(self):
        self.assertRegex(
            self.src,
            r'isConfigured\(\)|get\("binding"\)|DeviceCredentialStore',
            'BootReceiver must check that device is activated before starting',
        )


if __name__ == '__main__':
    unittest.main()
