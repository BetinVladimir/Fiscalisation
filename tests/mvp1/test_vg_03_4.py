"""VG-03.4 — Same operationId replay does not create duplicate line (journal deduplication)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMART = ROOT / 'SmartDevices'


class TestVG03_4(unittest.TestCase):
    def test_same_seq_replay_does_not_create_duplicate_line(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsSeqTest.same seq replay does not create duplicate line'],
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
