"""BUG-032.3 — startManagedRuntime logic lives in RuntimeHost, not MainActivity."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MAIN_ACT = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt'
RUNTIME_HOST = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/RuntimeHost.kt'


class TestBUG0323(unittest.TestCase):
    def setUp(self):
        self.assertTrue(RUNTIME_HOST.exists(), f'RuntimeHost.kt not found: {RUNTIME_HOST}')
        self.rh_src = RUNTIME_HOST.read_text()
        self.assertTrue(MAIN_ACT.exists(), f'MainActivity.kt not found: {MAIN_ACT}')
        self.ma_src = MAIN_ACT.read_text()

    def test_runtime_host_class_exists(self):
        self.assertRegex(
            self.rh_src,
            r'class\s+RuntimeHost\b',
            'RuntimeHost class must be defined',
        )

    def test_runtime_host_has_start_method(self):
        self.assertRegex(
            self.rh_src,
            r'fun\s+start\s*\(',
            'RuntimeHost must have a start() method',
        )

    def test_runtime_host_has_stop_method(self):
        self.assertRegex(
            self.rh_src,
            r'fun\s+stop\s*\(',
            'RuntimeHost must have a stop() method',
        )

    def test_runtime_host_manages_mqtt(self):
        self.assertIn(
            'BlueCashMqttRuntime',
            self.rh_src,
            'RuntimeHost must manage MQTT (BlueCashMqttRuntime)',
        )

    def test_runtime_host_does_not_use_run_on_ui_thread(self):
        self.assertNotIn(
            'runOnUiThread',
            self.rh_src,
            'RuntimeHost must not use runOnUiThread (Activity-only)',
        )


if __name__ == '__main__':
    unittest.main()
