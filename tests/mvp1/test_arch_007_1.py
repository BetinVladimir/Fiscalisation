"""ARCH-007.1 — BlueCashRuntime.kt exists with no android.* imports."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
RUNTIME_KT = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashRuntime.kt'
SMARTDEVICES = ROOT / 'SmartDevices'


class TestARCH007_1(unittest.TestCase):
    def test_bluecash_runtime_exists_and_pure_kotlin(self):
        self.assertTrue(RUNTIME_KT.exists(), f'BlueCashRuntime.kt missing: {RUNTIME_KT}')
        content = RUNTIME_KT.read_text()
        self.assertGreater(len(content.strip()), 0, 'BlueCashRuntime.kt is empty')
        for line in content.splitlines():
            self.assertNotIn(
                'import android',
                line,
                f'Found android import in BlueCashRuntime.kt: {line!r}',
            )

    def test_gradle_build_passes(self):
        result = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.AggregateRebuildTest'],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'Gradle rc=' + str(result.returncode) + '\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )


if __name__ == '__main__':
    unittest.main()
