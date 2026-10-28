"""ARCH-003.2 — split files must exist with correct package declarations."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
API = ROOT / "fiscal-backend" / "internal" / "api"

EXPECTED_FILES = [
    "middleware_idem.go",
    "middleware_http.go",
    "handlers_devices.go",
    "handlers_sales.go",
    "handlers_reports.go",
    "handlers_platform.go",
]


class TestArch003SplitFiles(unittest.TestCase):
    def test_split_files_exist(self):
        for name in EXPECTED_FILES:
            path = API / name
            self.assertTrue(path.exists(), f"split file missing: {path}")

    def test_split_files_declare_package_api(self):
        for name in EXPECTED_FILES:
            path = API / name
            if not path.exists():
                self.skipTest(f"{name} missing")
            first_line = path.read_text().splitlines()[0]
            self.assertEqual(
                first_line, "package api",
                f"{name}: first line must be 'package api', got {first_line!r}",
            )


if __name__ == "__main__":
    unittest.main()
