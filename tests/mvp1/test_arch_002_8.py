"""ARCH-002.8: money/validation helpers moved to money.go."""
import subprocess, pathlib

ROOT = pathlib.Path(__file__).parents[2]
DOMAIN = ROOT / "fiscal-backend/internal/domain"


def test_money_helpers_in_money_go():
    src = (DOMAIN / "money.go").read_text()
    for sym in [
        "func validTax(",
        "func validMoney(",
        "func validQuantity(",
        "func parseFixed(",
        "func lineTotalCents(",
        "func discountedLineTotalCents(",
        "func validDiscount(",
        "func saleTotal(",
        "func formatFixed(",
        "func contains(",
    ]:
        assert sym in src, f"money.go must contain {sym}"


def test_money_helpers_not_in_service_go():
    src = (DOMAIN / "service.go").read_text()
    for sym in [
        "func validTax(",
        "func validMoney(",
        "func validQuantity(",
        "func parseFixed(",
        "func lineTotalCents(",
        "func saleTotal(",
        "func formatFixed(",
    ]:
        assert sym not in src, f"service.go must NOT contain {sym}"


def test_go_build_and_tests():
    result = subprocess.run(
        ["go", "test", "./internal/domain/...", "-count=1"],
        cwd=ROOT / "fiscal-backend",
        capture_output=True, text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
