"""ARCH-003.6 — routes_snapshot_test.go and golden file exist."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
API = ROOT / "fiscal-backend" / "internal" / "api"


class TestArch003Snapshot(unittest.TestCase):
    def test_snapshot_test_exists(self):
        path = API / "routes_snapshot_test.go"
        self.assertTrue(path.exists(), f"routes_snapshot_test.go not found at {path}")
        content = path.read_text()
        self.assertIn("TestRouteTableSnapshot", content)

    def test_golden_file_exists(self):
        path = API / "testdata" / "routes.golden"
        self.assertTrue(path.exists(), f"routes.golden not found at {path}")
        routes = [l.strip() for l in path.read_text().splitlines() if l.strip()]
        self.assertGreater(len(routes), 30, "golden file should have >30 routes")
        self.assertIn("/public/v1/sales", routes)
        self.assertIn("/metrics", routes)
        self.assertIn("/public/v1/exports", routes)


if __name__ == "__main__":
    unittest.main()
