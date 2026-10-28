"""VG-03.2 — SequenceStore survives restart (persists SEQ across DatecsSocketProtocol instances)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMART = ROOT / 'SmartDevices'


class TestVG03_2(unittest.TestCase):
    def test_sequence_survives_restart(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsSeqTest.sequence survives restart'],
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
