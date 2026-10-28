"""ARCH-003.5 — handler functions live in correct split files."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
API = ROOT / "fiscal-backend" / "internal" / "api"

DEVICE_HANDLERS = ["manufacturingDeviceRegister", "platformDevices", "platformDevice",
                   "deviceActivationLookup", "deviceBootstrapChallenge",
                   "deviceBootstrapCreate", "deviceBootstrapCredential",
                   "deviceActivationConfirmation", "workstation", "func (h *Handler) device("]

SALES_HANDLERS = ["func (h *Handler) sales(", "func (h *Handler) sale(",
                  "func (h *Handler) operations(", "func (h *Handler) operation(",
                  "func (h *Handler) shifts(", "func (h *Handler) shift(",
                  "func (h *Handler) register("]

REPORT_HANDLERS = ["func (h *Handler) reports(", "func (h *Handler) report(",
                   "func (h *Handler) auditEvents(", "func (h *Handler) exports(",
                   "func (h *Handler) periodizedExports(", "func (h *Handler) export("]

PLATFORM_HANDLERS = ["func (h *Handler) webhookEndpoints(", "func (h *Handler) taxGroups(",
                     "func (h *Handler) organization(", "func (h *Handler) live(",
                     "func (h *Handler) ready(", "func (h *Handler) ping("]


class TestArch003HandlerPlacement(unittest.TestCase):
    def _check(self, filename, symbols):
        path = API / filename
        self.assertTrue(path.exists())
        content = path.read_text()
        for sym in symbols:
            self.assertIn(sym, content, f"{filename} missing {sym!r}")

    def test_device_handlers_in_handlers_devices(self):
        self._check("handlers_devices.go", DEVICE_HANDLERS)

    def test_sales_handlers_in_handlers_sales(self):
        self._check("handlers_sales.go", SALES_HANDLERS)

    def test_report_handlers_in_handlers_reports(self):
        self._check("handlers_reports.go", REPORT_HANDLERS)

    def test_platform_handlers_in_handlers_platform(self):
        self._check("handlers_platform.go", PLATFORM_HANDLERS)


if __name__ == "__main__":
    unittest.main()
