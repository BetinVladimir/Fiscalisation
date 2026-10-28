"""VG-02.4 — DatecsErrorCatalog parses negative error code category (PAPER)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMART = ROOT / 'SmartDevices'


class TestVG02_4(unittest.TestCase):
    def test_ok_parses_negative_error_code_category(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsErrorCatalogTest.ok parses negative error code category'],
            cwd=str(SMART),
            capture_output=True,
            text=True,
            timeout=180,
        )
        self.assertEqual(
            r.returncode, 0,
            f'gradle exited {r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1000:]}',
        )


if __name__ == '__main__':
    unittest.main()
