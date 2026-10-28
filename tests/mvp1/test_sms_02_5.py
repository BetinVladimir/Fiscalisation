"""SMS-02.5 — shared-runtime MQTT/BLE/HTTP ingress and foreground lifecycle.

Acceptance:
  - TransportIngress.kt and IngressDispatcher exist in shared-runtime
  - IngressDispatcher tests pass: two transports → one execute; restart preserves PENDING
  - bluecash-app characteristic tests still pass (BC50 regression unchanged)
  - shared-runtime unit tests pass with non-zero count
"""
import os
import subprocess
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"

SHARED_RUNTIME_SRC = SMARTDEVICES / "shared-runtime/src/main/kotlin/com/beeloy/fiscal/runtime"


class TestSMS02_5(unittest.TestCase):
    def test_transport_ingress_interface_exists(self):
        """TransportIngress.kt must exist in shared-runtime with required symbols."""
        f = SHARED_RUNTIME_SRC / "TransportIngress.kt"
        self.assertTrue(f.exists(), f"TransportIngress.kt not found: {f}")
        content = f.read_text()
        self.assertIn("TransportIngress", content, "Must define TransportIngress interface")
        self.assertIn("IngressDispatcher", content, "Must define IngressDispatcher")
        # Core contract: two transports → one execute
        self.assertIn("dispatch", content, "IngressDispatcher must have dispatch method")

    def test_ingress_dispatcher_deduplication_documented(self):
        """IngressDispatcher must document the two-transports→one-execute contract."""
        f = SHARED_RUNTIME_SRC / "TransportIngress.kt"
        content = f.read_text()
        self.assertIn("beginOperation", content,
                      "Must use beginOperation for deduplication (invariant 4)")

    def test_ingress_dispatcher_test_exists(self):
        """IngressDispatcherTest.kt must exist with tests covering key scenarios."""
        test_f = (
            SMARTDEVICES
            / "shared-runtime/src/test/kotlin/com/beeloy/fiscal/runtime/IngressDispatcherTest.kt"
        )
        self.assertTrue(test_f.exists(), f"IngressDispatcherTest.kt not found: {test_f}")
        content = test_f.read_text()
        # Must cover: two-transports dedup (method starts with lowercase)
        self.assertIn("twoTransports", content,
                      "Must have two-transports deduplication test")
        # Must cover: restart recovery (PENDING survives)
        self.assertIn("pending", content.lower(), "Must have restart/pending recovery test")
        # Must cover: UNKNOWN on failure (invariant 2)
        self.assertIn("UNKNOWN", content, "Must cover UNKNOWN state on execute failure (invariant 2)")

    def test_shared_runtime_tests_pass_with_ingress(self):
        """./gradlew :shared-runtime:test exits 0 including IngressDispatcherTest."""
        result = subprocess.run(
            ["./gradlew", ":shared-runtime:test", "--no-daemon"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-02-5-shared-runtime"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"shared-runtime test rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )

        # Verify IngressDispatcherTest was included in the run
        xml_dir = SMARTDEVICES / "shared-runtime/build/test-results"
        ingress_tests = 0
        for xml_file in xml_dir.rglob("*.xml"):
            try:
                tree = ET.parse(str(xml_file))
                suite = tree.getroot()
                if "IngressDispatcher" in suite.get("name", ""):
                    ingress_tests += int(suite.get("tests", "0"))
            except Exception:
                pass
        self.assertGreater(ingress_tests, 0,
                           "IngressDispatcherTest must have at least 1 executed test")

    def test_bluecash_app_characteristic_tests_unchanged(self):
        """BlueCash BC50 characteristic tests must still pass (BC50 regression check)."""
        result = subprocess.run(
            [
                "./gradlew",
                ":bluecash-app:testProdDebugUnitTest",
                "--tests", "*CommandProcessorCharacteristic*",
                "--no-daemon",
            ],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-02-5-regression"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"BC50 characteristic tests failed: rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )


if __name__ == "__main__":
    unittest.main()
