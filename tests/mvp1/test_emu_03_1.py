"""EMU-03.1 — FakeSmartFiscalServer: OK/OK_WITH_DOC/ERROR/DROP_AFTER_BODY modes + validation."""
import os
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SMART_DEVICES = ROOT / 'SmartDevices'


from native_harness import check_native_tests


class TestEMU03_1(unittest.TestCase):
    def test_fake_smart_fiscal_server_modes(self):
        check_native_tests(
            self,
            argv=[
                './gradlew', ':daisy-smart-app:testProdDebugUnitTest',
                '--tests', 'com.beeloy.fiscal.daisy.FakeSmartFiscalServerTest',
                '--rerun-tasks',
            ],
            root=SMART_DEVICES,
            cwd='.',
            minimum=4,
            timeout=360,
        )

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out = Path(artifact_dir) / 'emu-03-evidence'
            out.mkdir(parents=True, exist_ok=True)
            daisy_results = SMART_DEVICES / 'daisy-smart-app/build/test-results'
            if daisy_results.exists():
                for xml in daisy_results.rglob('TEST-*.xml'):
                    (out / xml.name).write_bytes(xml.read_bytes())


if __name__ == '__main__':
    unittest.main()
