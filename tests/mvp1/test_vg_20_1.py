"""VG-20.1 — All golden vectors from datecs_golden_vectors.json verified in Kotlin host test."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
RUNNER = ROOT / "SmartDevices" / "bluecash-app" / "tests" / "host" / "run-datecs-codec.py"
VECTORS = ROOT / "docs" / "MVP1" / "vendor" / "datecs_golden_vectors.json"


class TestVG201GoldenVectorsKotlin(unittest.TestCase):
    def test_runner_exists(self):
        self.assertTrue(RUNNER.exists(), f"run-datecs-codec.py not found at {RUNNER}")

    def test_golden_vectors_file_exists(self):
        self.assertTrue(VECTORS.exists(), f"datecs_golden_vectors.json not found at {VECTORS}")

    def test_vectors_have_11_requests_and_2_answers(self):
        import json
        with open(VECTORS) as f:
            vecs = json.load(f)
        requests = [k for k, v in vecs.items() if "request_hex" in v]
        answers = [k for k, v in vecs.items() if "response_hex" in v]
        self.assertEqual(len(requests), 11,
                         f"Expected 11 request vectors, got {len(requests)}: {requests}")
        self.assertEqual(len(answers), 2,
                         f"Expected 2 answer vectors, got {len(answers)}: {answers}")

    def test_kotlin_host_all_golden_vectors_pass(self):
        """Run the Kotlin DatecsCodecVendorHost and verify all golden vector tests pass."""
        result = subprocess.run(
            ["python3", str(RUNNER)],
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            f"Kotlin golden vector host test failed.\nstdout: {result.stdout}\nstderr: {result.stderr}"
        )
        self.assertIn(
            "actual Kotlin Datecs codec host tests: OK",
            result.stdout,
            f"Expected 'actual Kotlin Datecs codec host tests: OK' in output.\nstdout: {result.stdout}"
        )


if __name__ == "__main__":
    unittest.main()
