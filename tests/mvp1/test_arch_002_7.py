"""ARCH-002.7: sale_service.go extracted from service.go."""
import subprocess, re, pathlib

ROOT = pathlib.Path(__file__).parents[2]
DOMAIN = ROOT / "fiscal-backend/internal/domain"


def test_sale_service_file_exists():
    assert (DOMAIN / "sale_service.go").exists(), "sale_service.go must exist"


def test_sale_functions_in_sale_service():
    src = (DOMAIN / "sale_service.go").read_text()
    for sym in [
        "func (s *Service) CreateSale(",
        "func (s *Service) OpenSaleWithFirstLine(",
        "func (s *Service) AddLine(",
        "func (s *Service) PayForTenant(",
        "func (s *Service) FinalizeSaleForTenant(",
        "func (s *Service) Receipt(",
    ]:
        assert sym in src, f"sale_service.go must contain {sym}"


def test_sale_functions_not_in_service_go():
    src = (DOMAIN / "service.go").read_text()
    for sym in [
        "func (s *Service) CreateSale(",
        "func (s *Service) OpenSaleWithFirstLine(",
        "func (s *Service) PayForTenant(",
        "func (s *Service) FinalizeSaleForTenant(",
    ]:
        assert sym not in src, f"service.go must NOT contain {sym}"


def test_service_go_under_300_lines():
    lines = len((DOMAIN / "service.go").read_text().splitlines())
    assert lines <= 300, f"service.go must be ≤300 lines, got {lines}"


def test_go_build_and_tests():
    result = subprocess.run(
        ["go", "test", "./internal/domain/...", "-count=1"],
        cwd=ROOT / "fiscal-backend",
        capture_output=True, text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
