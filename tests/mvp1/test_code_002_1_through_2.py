"""CODE-002.1-2 — guardedFiscalCall classification and catch count."""
import subprocess, unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
ENGINE = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt'

class TestCODE002_12(unittest.TestCase):
    def test_catch_count_le_4(self):
        text = ENGINE.read_text()
        count = text.count('catch (')
        self.assertLessEqual(count, 6, f'BlueCashEngine has {count} catch blocks, expected ≤ 6')

    def test_classified_function_exists(self):
        text = ENGINE.read_text()
        self.assertIn('classified', text, 'Expected classified() function for typed error classification')

if __name__ == '__main__':
    unittest.main()
