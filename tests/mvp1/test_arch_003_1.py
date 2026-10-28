"""ARCH-003.1 — handler.go must be ≤500 lines after the split."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
HANDLER = ROOT / "fiscal-backend" / "internal" / "api" / "handler.go"


class TestArch003HandlerSize(unittest.TestCase):
    def test_handler_go_under_500_lines(self):
        self.assertTrue(HANDLER.exists(), f"handler.go not found at {HANDLER}")
        lines = HANDLER.read_text().splitlines()
        self.assertLessEqual(
            len(lines), 500,
            f"handler.go is {len(lines)} lines; must be ≤500 after ARCH-003 split",
        )


if __name__ == "__main__":
    unittest.main()
