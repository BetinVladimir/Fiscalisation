"""CODE-001.1 — No e.message in setOf() pattern in Kotlin sources."""
import subprocess, unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
SRC = ROOT / 'SmartDevices/bluecash-app/src/main'

class TestCODE001_1(unittest.TestCase):
    def test_no_message_in_setof(self):
        r = subprocess.run(
            ['grep', '-rn', 'e.message in', str(SRC), '--include=*.kt'],
            capture_output=True, text=True)
        self.assertEqual(r.stdout.strip(), '',
            f'Found e.message in setOf pattern (should use when(e)):\n{r.stdout}')

if __name__ == '__main__':
    unittest.main()
