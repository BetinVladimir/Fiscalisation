"""ARCH-003.4 — HTTP helpers in middleware_http.go, not handler.go."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
API = ROOT / "fiscal-backend" / "internal" / "api"

HTTP_SYMBOLS = ["func write(", "func problem(", "func cors(", "func recoverer(",
                "func version(", "func SignWebhook(", "func readBody(",
                "func tenantID(", "func actorSubject(", "func replayKey(",
                "func paginate[", "func optionalTime(", "func ifMatch(",
                "func decodeStrict("]


class TestArch003HttpPlacement(unittest.TestCase):
    def test_http_helpers_in_middleware_http(self):
        target = API / "middleware_http.go"
        self.assertTrue(target.exists())
        content = target.read_text()
        for sym in HTTP_SYMBOLS:
            self.assertIn(sym, content, f"middleware_http.go missing {sym!r}")

    def test_http_helpers_not_in_handler(self):
        handler = API / "handler.go"
        content = handler.read_text()
        for sym in ["func write(", "func problem(", "func cors(", "func recoverer("]:
            self.assertNotIn(sym, content, f"handler.go must not define {sym!r}")


if __name__ == "__main__":
    unittest.main()
