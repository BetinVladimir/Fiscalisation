"""TST-GO-11.1 — TestConcurrentFinalizeSameKeyProducesOneOperation (TG-001)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestTSTGO11(unittest.TestCase):
    def test_concurrent_finalize_same_key_one_operation(self):
        check_native_tests(
            self,
            ['go', 'test', '-json', '-race',
             './internal/domain/',
             '-run', '^(TestConcurrentFinalizeSameKeyProducesOneOperation'
                     '|TestRepositoryReloadPreservesState'
                     '|TestEmptyRepositoryStartsEmpty)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestConcurrentFinalizeSameKeyProducesOneOperation',
                'TestRepositoryReloadPreservesState',
                'TestEmptyRepositoryStartsEmpty',
            ),
            minimum=3,
        )


if __name__ == '__main__':
    unittest.main()
