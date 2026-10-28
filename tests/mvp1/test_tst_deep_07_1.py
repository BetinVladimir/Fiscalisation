"""TST-DEEP-07.1 — webhook race before/after link, 0 rows → not lost (test executes)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTDEEP07_1(unittest.TestCase):
    def test_webhook_race_before_link_not_lost(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestWebhookBeforeLinkIsNotLostAfterZeroAffectedRows'
                     '|TestWebhookLinkRaceArrivesBeforeAndAfterLink)$',
             '-count=1', '-timeout=60s'],
            root=ROOT,
            cwd='minipos/beeminipos-backend',
            required_tests=(
                'TestWebhookBeforeLinkIsNotLostAfterZeroAffectedRows',
                'TestWebhookLinkRaceArrivesBeforeAndAfterLink',
            ),
            minimum=2,
            timeout=120,
        )


if __name__ == '__main__':
    unittest.main()
