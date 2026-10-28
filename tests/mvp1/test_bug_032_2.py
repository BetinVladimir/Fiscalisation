"""BUG-032.2 — BlueCashRuntimeService is a foreground Service that calls RuntimeHost.start()."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SVC = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashRuntimeService.kt'


class TestBUG0322(unittest.TestCase):
    def setUp(self):
        self.assertTrue(SVC.exists(), f'BlueCashRuntimeService.kt not found: {SVC}')
        self.src = SVC.read_text()

    def test_class_extends_service(self):
        self.assertRegex(
            self.src,
            r'class\s+BlueCashRuntimeService\s*:\s*Service\b',
            'BlueCashRuntimeService must extend Service',
        )

    def test_on_start_command_calls_start_foreground(self):
        self.assertIn(
            'startForeground(',
            self.src,
            'onStartCommand must call startForeground()',
        )

    def test_on_start_command_calls_runtime_host_start(self):
        self.assertRegex(
            self.src,
            r'RuntimeHost\b.*\.start\(|\.start\(.*binding',
            'onStartCommand must create RuntimeHost and call start()',
        )

    def test_on_start_command_declared(self):
        self.assertIn(
            'override fun onStartCommand(',
            self.src,
            'onStartCommand must be overridden',
        )

    def test_on_destroy_stops_runtime(self):
        self.assertIn(
            'override fun onDestroy()',
            self.src,
            'onDestroy must be overridden',
        )
        destroy_idx = self.src.index('override fun onDestroy()')
        destroy_block = self.src[destroy_idx:destroy_idx + 300]
        self.assertRegex(
            destroy_block,
            r'runtimeHost.*stop\(\)|\.stop\(\)',
            'onDestroy must stop the runtimeHost',
        )


if __name__ == '__main__':
    unittest.main()
