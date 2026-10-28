"""VG-20.4 — scripts/check-golden-vectors.sh: golden vector files exist and have test cases."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "scripts" / "check-golden-vectors.sh"
FIXTURES = ROOT / "docs" / "MVP1" / "fixtures"
VECTORS_JSON = ROOT / "docs" / "MVP1" / "vendor" / "datecs_golden_vectors.json"


class TestVG204CheckGoldenVectors(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"check-golden-vectors.sh not found at {SCRIPT}")

    def test_datecs_golden_vectors_json_exists(self):
        self.assertTrue(VECTORS_JSON.exists(),
                        f"datecs_golden_vectors.json not found at {VECTORS_JSON}")

    def test_datecs_golden_vectors_has_13_entries(self):
        import json
        with open(VECTORS_JSON) as f:
            vecs = json.load(f)
        self.assertEqual(len(vecs), 13,
                         f"datecs_golden_vectors.json must have 13 entries, got {len(vecs)}")

    def test_check_golden_vectors_script_passes(self):
        result = subprocess.run(
            ["bash", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            f"check-golden-vectors.sh must exit 0.\nstdout: {result.stdout}\nstderr: {result.stderr}"
        )


if __name__ == "__main__":
    unittest.main()
