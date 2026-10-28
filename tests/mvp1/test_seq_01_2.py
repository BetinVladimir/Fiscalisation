"""SEQ-01.2 — Full serial suite including profile-specific SEQ cycle fix."""
from pathlib import Path
import sys, unittest
ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "docs/MVP1/fixes"))
from native_harness import check_native_tests


class TestSEQ01_2(unittest.TestCase):
    def test_native(self):
        check_native_tests(
            self,
            ['bash', 'IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh'],
            root=ROOT,
            cwd='.',
            markers=('ok 1 - Datecs cycle96', 'ok 2 - Daisy cycle224'),
            timeout=120,
        )


if __name__ == '__main__':
    unittest.main()
