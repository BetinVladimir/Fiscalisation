"""BUG-032.1 — AndroidManifest.xml has required permissions, service, and receiver."""
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MANIFEST = ROOT / 'SmartDevices/bluecash-app/src/main/AndroidManifest.xml'
NS = 'http://schemas.android.com/apk/res/android'

REQUIRED_PERMISSIONS = [
    'android.permission.FOREGROUND_SERVICE',
    'android.permission.FOREGROUND_SERVICE_CONNECTED_DEVICE',
    'android.permission.RECEIVE_BOOT_COMPLETED',
    'android.permission.POST_NOTIFICATIONS',
    'android.permission.WAKE_LOCK',
]


class TestBUG0321(unittest.TestCase):
    def setUp(self):
        self.assertTrue(MANIFEST.exists(), f'AndroidManifest.xml not found: {MANIFEST}')
        self.tree = ET.parse(str(MANIFEST))
        self.root = self.tree.getroot()

    def _permissions(self):
        return {
            el.get(f'{{{NS}}}name')
            for el in self.root.iter('uses-permission')
        }

    def _application(self):
        app = self.root.find('application')
        self.assertIsNotNone(app, '<application> element missing from manifest')
        return app

    def test_required_permissions_present(self):
        perms = self._permissions()
        for p in REQUIRED_PERMISSIONS:
            self.assertIn(p, perms, f'Missing permission: {p}')

    def test_service_bluecashruntimeservice_declared(self):
        app = self._application()
        services = [el for el in app.findall('service')]
        names = [s.get(f'{{{NS}}}name', '') for s in services]
        matching = [n for n in names if 'BlueCashRuntimeService' in n]
        self.assertGreater(
            len(matching), 0,
            f'<service android:name="...BlueCashRuntimeService"> not found; services: {names}',
        )
        svc = next(s for s in services if 'BlueCashRuntimeService' in s.get(f'{{{NS}}}name', ''))
        fst = svc.get(f'{{{NS}}}foregroundServiceType', '')
        self.assertIn(
            'connectedDevice', fst,
            f'foregroundServiceType must include connectedDevice; got: {fst!r}',
        )

    def test_bootreceiver_declared(self):
        app = self._application()
        receivers = [el for el in app.findall('receiver')]
        names = [r.get(f'{{{NS}}}name', '') for r in receivers]
        matching = [n for n in names if 'BootReceiver' in n]
        self.assertGreater(
            len(matching), 0,
            f'<receiver android:name="...BootReceiver"> not found; receivers: {names}',
        )
        rec = next(r for r in receivers if 'BootReceiver' in r.get(f'{{{NS}}}name', ''))
        actions = [
            f.get(f'{{{NS}}}name', '')
            for f in rec.findall('intent-filter/action')
        ]
        self.assertIn(
            'android.intent.action.BOOT_COMPLETED', actions,
            f'BootReceiver intent-filter must include BOOT_COMPLETED; got: {actions}',
        )


if __name__ == '__main__':
    unittest.main()
