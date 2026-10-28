"""CODE-001.2 — DatecsFailure sealed hierarchy used for classification."""
import subprocess, unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

class TestCODE001_2(unittest.TestCase):
    def test_datecs_failure_hierarchy_exists(self):
        r = subprocess.run(
            ['grep', '-rn', 'is DatecsFailure', str(SMARTDEVICES / 'bluecash-app/src/main'), '--include=*.kt'],
            capture_output=True, text=True)
        self.assertGreater(len(r.stdout.strip()), 0,
            'Expected "is DatecsFailure" pattern for typed error classification')

if __name__ == '__main__':
    unittest.main()
