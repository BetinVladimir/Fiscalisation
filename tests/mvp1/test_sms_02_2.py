"""SMS-02.2 — shared-runtime module created; ./gradlew :shared-runtime:assemble exits 0."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"

REQUIRED_SOURCE_FILES = (
    "shared-runtime/src/main/kotlin/com/beeloy/fiscal/runtime/OperationJournal.kt",
    "shared-runtime/src/main/kotlin/com/beeloy/fiscal/runtime/CommandProcessor.kt",
    "shared-runtime/src/main/kotlin/com/beeloy/fiscal/runtime/SecretVault.kt",
    "shared-runtime/build.gradle.kts",
)

REQUIRED_SETTINGS_ENTRY = ":shared-runtime"


class TestSMS02_2(unittest.TestCase):
    def test_shared_runtime_module_assembles(self):
        # Verify settings.gradle.kts includes :shared-runtime
        settings_file = SMARTDEVICES / "settings.gradle.kts"
        self.assertTrue(settings_file.exists(), f"settings.gradle.kts not found at {settings_file}")
        settings_text = settings_file.read_text()
        self.assertIn(
            REQUIRED_SETTINGS_ENTRY, settings_text,
            f"{REQUIRED_SETTINGS_ENTRY!r} not found in settings.gradle.kts",
        )

        # Verify required source files exist
        for rel_path in REQUIRED_SOURCE_FILES:
            full = SMARTDEVICES / rel_path
            self.assertTrue(full.exists(), f"Required file missing: {rel_path}")

        # Run :shared-runtime:assemble and verify exit 0
        result = subprocess.run(
            ["./gradlew", ":shared-runtime:assemble", "--no-daemon"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-02-2"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"./gradlew :shared-runtime:assemble rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )

    def test_shared_runtime_operation_journal_interface_fields(self):
        """OperationJournal.kt must define PENDING, COMMITTED, FISCAL_PENDING, UNKNOWN states."""
        journal_file = (
            SMARTDEVICES
            / "shared-runtime/src/main/kotlin/com/beeloy/fiscal/runtime/OperationJournal.kt"
        )
        self.assertTrue(journal_file.exists(), "OperationJournal.kt not found")
        content = journal_file.read_text()

        for required in ("PENDING", "COMMITTED", "FISCAL_PENDING", "UNKNOWN",
                         "beginOperation", "markUnknown", "markFiscalPending"):
            self.assertIn(
                required, content,
                f"OperationJournal.kt missing required symbol: {required!r}",
            )

    def test_daisy_smart_app_depends_on_shared_runtime(self):
        """daisy-smart-app/build.gradle.kts must declare implementation(project(':shared-runtime'))."""
        daisy_build = SMARTDEVICES / "daisy-smart-app/build.gradle.kts"
        self.assertTrue(daisy_build.exists(), "daisy-smart-app/build.gradle.kts not found")
        content = daisy_build.read_text()
        self.assertIn(
            ":shared-runtime", content,
            "daisy-smart-app/build.gradle.kts must reference :shared-runtime",
        )


if __name__ == "__main__":
    unittest.main()
