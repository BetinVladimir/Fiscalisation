"""CODE-005.1 — acceptance: OperationState type with Terminal() method exists in domain package."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestCODE0051(unittest.TestCase):
    def test_operation_state_terminal_set(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestOperationStateTerminalSet$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestOperationStateTerminalSet',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
