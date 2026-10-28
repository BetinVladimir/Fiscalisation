"""VG-06.3 — Invalid plan (total mismatch) does not charge card or open receipt."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMART = ROOT / 'SmartDevices'


class TestVG06_3(unittest.TestCase):
    def test_invalid_plan_does_not_charge_card(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.BlueCashValidationTest.invalid plan does not charge card'],
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
