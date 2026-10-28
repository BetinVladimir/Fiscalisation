"""ARCH-003.3 — idempotency middleware functions only in middleware_idem.go."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
API = ROOT / "fiscal-backend" / "internal" / "api"

IDEM_SYMBOLS = [
    "fiscalIdempotencyOwnerKey",
    "fiscalCapturedResponse",
    "fiscalIdempotency",
    "fiscalReplayHeaders",
    "applyFiscalReplayHeaders",
    "fiscalIdempotencyFingerprint",
    "saveReplay",
    "saveEmptyReplay",
]


class TestArch003IdemPlacement(unittest.TestCase):
    def test_idem_symbols_in_middleware_idem(self):
        target = API / "middleware_idem.go"
        self.assertTrue(target.exists())
        content = target.read_text()
        for sym in IDEM_SYMBOLS:
            self.assertIn(sym, content, f"middleware_idem.go missing symbol {sym!r}")

    def test_idem_symbols_not_in_handler(self):
        handler = API / "handler.go"
        self.assertTrue(handler.exists())
        content = handler.read_text()
        for sym in ["fiscalIdempotencyOwnerKey", "fiscalCapturedResponse",
                    "applyFiscalReplayHeaders", "fiscalIdempotencyFingerprint"]:
            self.assertNotIn(
                sym, content,
                f"handler.go must not define {sym!r} after split",
            )


if __name__ == "__main__":
    unittest.main()
