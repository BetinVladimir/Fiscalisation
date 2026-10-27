"""TST-GO-12.1 — TestFiscalUNPAllocator* (TG-002)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO12(unittest.TestCase):
    def test_unp_allocator(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestFiscalUNPAllocator',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestFiscalUNPAllocatorConcurrentDisjointRanges',
                'TestFiscalUNPAllocatorIdentityImmutable',
                'TestFiscalUNPAllocatorRejectsInvalidOrExhaustedWithoutMutation',
                'TestFiscalUNPAllocatorSaveAndReloadFailureRollsBack',
                'TestFiscalUNPAllocatorSharedOnlineAndOfflineSerialLedger',
            ),
            minimum=5,
        )


if __name__ == '__main__':
    unittest.main()
