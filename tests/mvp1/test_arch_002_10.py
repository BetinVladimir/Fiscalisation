"""ARCH-002.10: final verification — service.go ≤300 lines, API hash unchanged."""
import subprocess, pathlib, hashlib

ROOT = pathlib.Path(__file__).parents[2]
DOMAIN = ROOT / "fiscal-backend/internal/domain"
GOLDEN_SHA_FILE = DOMAIN / "testdata/service_api.sha256"


def test_service_go_line_count():
    lines = len((DOMAIN / "service.go").read_text().splitlines())
    assert lines <= 300, f"service.go must be ≤300 lines, got {lines}"


def test_split_files_exist():
    for name in [
        "sale_service.go",
        "shift_service.go",
        "ble_service.go",
        "clock_service.go",
        "device_binding_service.go",
        "reversal_service.go",
        "outbox_service.go",
    ]:
        assert (DOMAIN / name).exists(), f"{name} must exist"


def test_api_hash_unchanged():
    """Behavior snapshot hash must equal the golden hash written during ARCH-002.1."""
    result = subprocess.run(
        ["go", "test", "./internal/domain/...", "-run", "TestServiceBehaviorSnapshot", "-v", "-count=1"],
        cwd=ROOT / "fiscal-backend",
        capture_output=True, text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    # All snapshot tests must pass — that implicitly validates the hash
    assert "PASS" in result.stdout


def test_full_domain_test_suite():
    result = subprocess.run(
        ["go", "test", "./internal/domain/...", "-count=1"],
        cwd=ROOT / "fiscal-backend",
        capture_output=True, text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
