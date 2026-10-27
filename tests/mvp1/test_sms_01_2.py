"""SMS-01.2 — DAISY_SMART_PROTOCOLS.md has 14 vendor questions Q-S1..Q-S8 + Q-P1..Q-P6."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestSMS01_2(unittest.TestCase):
    def test_daisy_smart_protocols_questions(self):
        protocols_path = ROOT / 'docs' / 'vendor' / 'DAISY_SMART_PROTOCOLS.md'
        self.assertTrue(protocols_path.exists(), 'DAISY_SMART_PROTOCOLS.md not found')
        content = protocols_path.read_text(encoding='utf-8')

        # Count unique question IDs
        question_ids = set(re.findall(r'Q-[SP]\d+', content))
        self.assertGreaterEqual(len(question_ids), 14,
                                f'Expected >= 14 question IDs, found {len(question_ids)}: {sorted(question_ids)}')

        # Must have both Q-S and Q-P series
        qs = {q for q in question_ids if q.startswith('Q-S')}
        qp = {q for q in question_ids if q.startswith('Q-P')}
        self.assertGreaterEqual(len(qs), 8, f'Expected >= 8 Q-S questions, got {len(qs)}')
        self.assertGreaterEqual(len(qp), 6, f'Expected >= 6 Q-P questions, got {len(qp)}')


if __name__ == '__main__':
    unittest.main()
