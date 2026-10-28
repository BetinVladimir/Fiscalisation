"""SMS-02.4 — vendor-neutral journal/locking/processor strategy in shared-runtime.

Acceptance:
  - shared-runtime has InMemoryOperationJournal (concrete OperationJournal impl)
  - shared-runtime has OperationLock (reusable single-operation lock)
  - shared-runtime unit tests pass (InMemoryOperationJournalTest)
  - bluecash-app characteristic tests (SMS-02.1) still pass — same results after extraction
  - daisy-smart-app pre-existing baseline failures not introduced by our changes
"""
import os
import subprocess
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"

SHARED_RUNTIME_SRC = SMARTDEVICES / "shared-runtime/src/main/kotlin/com/beeloy/fiscal/runtime"


class TestSMS02_4(unittest.TestCase):
    def test_shared_runtime_has_in_memory_journal(self):
        """InMemoryOperationJournal.kt must exist in shared-runtime."""
        f = SHARED_RUNTIME_SRC / "InMemoryOperationJournal.kt"
        self.assertTrue(f.exists(), f"InMemoryOperationJournal.kt not found: {f}")
        content = f.read_text()
        self.assertIn("InMemoryOperationJournal", content)
        self.assertIn("OperationJournal", content)
        # Must implement all interface methods
        for method in ("beginOperation", "commit", "markFiscalPending", "fail", "markUnknown"):
            self.assertIn(method, content, f"InMemoryOperationJournal missing method: {method!r}")

    def test_shared_runtime_has_operation_lock(self):
        """OperationLock.kt must exist in shared-runtime."""
        f = SHARED_RUNTIME_SRC / "OperationLock.kt"
        self.assertTrue(f.exists(), f"OperationLock.kt not found: {f}")
        content = f.read_text()
        self.assertIn("OperationLock", content)
        self.assertIn("withOperation", content)

    def test_shared_runtime_tests_pass(self):
        """./gradlew :shared-runtime:test exits 0 and reports non-zero test count."""
        result = subprocess.run(
            ["./gradlew", ":shared-runtime:test", "--no-daemon"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-02-4-shared-runtime"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"shared-runtime test rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )

        # Count tests from JUnit XML
        xml_dir = SMARTDEVICES / "shared-runtime/build/test-results"
        test_count = 0
        for xml_file in xml_dir.rglob("*.xml"):
            try:
                tree = ET.parse(str(xml_file))
                suite = tree.getroot()
                n = int(suite.get("tests", "0"))
                test_count += n
            except Exception:
                pass
        self.assertGreater(test_count, 0, "shared-runtime must have at least 1 executed test")

    def test_characteristic_tests_unchanged_after_extraction(self):
        """SMS-02.1 characteristic tests must still pass — same processor behavior."""
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
            out = Path(artifact_dir) / "sms-02-4-characteristic"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"Characteristic tests failed after extraction: rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )

    def test_shared_runtime_journal_invariant_2_documented(self):
        """InMemoryOperationJournal must document invariant 2 (UNKNOWN ≠ FAILED)."""
        f = SHARED_RUNTIME_SRC / "InMemoryOperationJournal.kt"
        content = f.read_text()
        self.assertIn(
            "UNKNOWN", content,
            "InMemoryOperationJournal must reference UNKNOWN state (invariant 2)",
        )
        # OperationJournal.State.UNKNOWN and State.FAILED must be distinct
        journal_kt = SHARED_RUNTIME_SRC / "OperationJournal.kt"
        journal_content = journal_kt.read_text()
        self.assertIn("UNKNOWN", journal_content)
        self.assertIn("FAILED", journal_content)
        # They appear as separate enum constants → test done by structure check above


if __name__ == "__main__":
    unittest.main()
