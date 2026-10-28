"""VG-03.3 — First command after unknown SEQ is read-only (seqUnknown=true when store is empty)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMART = ROOT / 'SmartDevices'


class TestVG03_3(unittest.TestCase):
    def test_first_command_after_unknown_seq_is_read_only(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsSeqTest.first command after unknown seq is read only'],
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
